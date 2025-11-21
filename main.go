package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"net/textproto"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/miekg/dns"
)

type Result struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type HTTPCheck struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Path           string `json:"path"`
	Expected       string `json:"expected_content"`
	Regex          bool   `json:"regex"`
	Status         int    `json:"status"`
	Scheme         string `json:"scheme"`
	UsernameParam  string `json:"username_param"`
	PasswordParam  string `json:"password_param"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	AllowInsecure  bool   `json:"allow_insecure_tls"`
}

type SMTPCheck struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Body     string `json:"body"`
}

type POP3Command struct {
	Command string `json:"command"`
	Expected string `json:"expected"`
	Regex   bool   `json:"regex"`
}

type POP3Check struct {
	Host    string        `json:"host"`
	Port    int           `json:"port"`
	UseSSL  bool          `json:"use_ssl"`
	Username string       `json:"username"`
	Password string       `json:"password"`
	Commands []POP3Command `json:"commands"`
}

type FTPFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Regex  string `json:"regex"`
}

type FTPCheck struct {
	Host     string    `json:"host"`
	Port     int       `json:"port"`
	Username string    `json:"username"`
	Password string    `json:"password"`
	Files    []FTPFile `json:"files"`
}

type DNSRecord struct {
	Kind    string   `json:"kind"`
	Domain  string   `json:"domain"`
	Answers []string `json:"answers"`
}

type DNSCheck struct {
	Server  string      `json:"server"`
	Port    int         `json:"port"`
	Records []DNSRecord `json:"records"`
}

type Config struct {
	TimeoutSeconds int       `json:"timeout_seconds"`
	HTTP           HTTPCheck `json:"http"`
	HTTPS          HTTPCheck `json:"https"`
	SMTP           SMTPCheck `json:"smtp"`
	POP3           POP3Check `json:"pop3"`
	FTP            FTPCheck  `json:"ftp"`
	DNS            DNSCheck  `json:"dns"`
}

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.json"
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, Result{OK: true, Detail: "ok"})
	})

	http.HandleFunc("/check/http", withTimeout(timeout, func(ctx context.Context) Result {
		return checkHTTP(ctx, cfg.HTTP)
	}))

	http.HandleFunc("/check/https", withTimeout(timeout, func(ctx context.Context) Result {
		return checkHTTP(ctx, cfg.HTTPS)
	}))

	http.HandleFunc("/check/smtp", withTimeout(timeout, func(ctx context.Context) Result {
		return checkSMTP(ctx, cfg.SMTP)
	}))

	http.HandleFunc("/check/pop3", withTimeout(timeout, func(ctx context.Context) Result {
		return checkPOP3(ctx, cfg.PopSafe())
	}))

	http.HandleFunc("/check/ftp", withTimeout(timeout, func(ctx context.Context) Result {
		return checkFTP(ctx, cfg.FTP)
	}))

	http.HandleFunc("/check/dns", withTimeout(timeout, func(ctx context.Context) Result {
		return checkDNS(ctx, cfg.DNS)
	}))

	addr := ":8080"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func (c Config) PopSafe() POP3Check {
	// Ensure port defaults are present to avoid confusing zero values.
	clone := c.POP3
	if clone.Port == 0 {
		clone.Port = 110
	}
	return clone
}

func loadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func withTimeout(timeout time.Duration, fn func(context.Context) Result) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		res := fn(ctx)
		status := http.StatusOK
		if !res.OK {
			status = http.StatusServiceUnavailable
		}
		respondJSON(w, status, res)
	}
}

func respondJSON(w http.ResponseWriter, status int, res Result) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Printf("response encode error: %v", err)
	}
}

func checkHTTP(ctx context.Context, cfg HTTPCheck) Result {
	scheme := cfg.Scheme
	if scheme == "" {
		scheme = "http"
	}

	port := cfg.Port
	if port == 0 {
		if scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}

	path := cfg.Path
	if path == "" {
		path = "/"
	}

	query := url.Values{}
	if cfg.UsernameParam != "" && cfg.Username != "" {
		query.Set(cfg.UsernameParam, cfg.Username)
	}
	if cfg.PasswordParam != "" && cfg.Password != "" {
		query.Set(cfg.PasswordParam, cfg.Password)
	}
	if len(query) > 0 {
		if strings.Contains(path, "?") {
			path += "&" + query.Encode()
		} else {
			path += "?" + query.Encode()
		}
	}

	u := url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(port)),
		Path:   path,
	}

	transport := &http.Transport{}
	if scheme == "https" && cfg.AllowInsecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // scoring infra suele usar certs propios.
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   deadlineFromCtx(ctx),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("http request build error: %v", err)}
	}

	resp, err := client.Do(req)
	if err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("http request error: %v", err)}
	}
	defer resp.Body.Close()

	if cfg.Status > 0 && resp.StatusCode != cfg.Status {
		return Result{OK: false, Detail: fmt.Sprintf("status %d != expected %d", resp.StatusCode, cfg.Status)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("read body error: %v", err)}
	}

	if cfg.Expected != "" {
		if cfg.Regex {
			matched, err := regexp.Match(cfg.Expected, body)
			if err != nil {
				return Result{OK: false, Detail: fmt.Sprintf("regex error: %v", err)}
			}
			if !matched {
				return Result{OK: false, Detail: "body does not match regex"}
			}
		} else {
			if !strings.Contains(string(body), cfg.Expected) {
				return Result{OK: false, Detail: "body does not contain expected text"}
			}
		}
	}

	return Result{OK: true, Detail: fmt.Sprintf("%s check passed", strings.ToUpper(scheme))}
}

func checkSMTP(ctx context.Context, cfg SMTPCheck) Result {
	port := cfg.Port
	if port == 0 {
		port = 25
	}
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(port))

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("smtp dial error: %v", err)}
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("smtp client error: %v", err)}
	}
	defer client.Close()

	if err := client.Noop(); err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("smtp NOOP failed: %v", err)}
	}

	if err := client.Mail(cfg.Sender); err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("smtp MAIL failed: %v", err)}
	}
	if err := client.Rcpt(cfg.Receiver); err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("smtp RCPT failed: %v", err)}
	}

	dataWriter, err := client.Data()
	if err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("smtp DATA failed: %v", err)}
	}
	if _, err := dataWriter.Write([]byte(cfg.Body)); err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("smtp write failed: %v", err)}
	}
	if err := dataWriter.Close(); err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("smtp data close failed: %v", err)}
	}

	if err := client.Quit(); err != nil && !errorsIsClosed(err) {
		return Result{OK: false, Detail: fmt.Sprintf("smtp quit failed: %v", err)}
	}

	return Result{OK: true, Detail: "SMTP send/receive accepted"}
}

func checkPOP3(ctx context.Context, cfg POP3Check) Result {
	port := cfg.Port
	if port == 0 {
		port = 110
	}
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(port))

	dialer := net.Dialer{}
	var conn net.Conn
	var err error
	if cfg.UseSSL {
		conn, err = tls.DialWithDialer(&dialer, "tcp", address, &tls.Config{InsecureSkipVerify: true})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("pop3 dial error: %v", err)}
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	tp := textproto.NewConn(conn)
	defer tp.Close()

	// Greeting
	line, err := tp.ReadLine()
	if err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("pop3 greeting error: %v", err)}
	}
	if !strings.HasPrefix(line, "+OK") {
		return Result{OK: false, Detail: "pop3 greeting not OK"}
	}

	if resp, err := pop3Cmd(tp, "USER "+cfg.Username); err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("pop3 USER error: %v", err)}
	} else if !strings.HasPrefix(resp, "+OK") {
		return Result{OK: false, Detail: "pop3 USER not accepted"}
	}
	if resp, err := pop3Cmd(tp, "PASS "+cfg.Password); err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("pop3 PASS error: %v", err)}
	} else if !strings.HasPrefix(resp, "+OK") {
		return Result{OK: false, Detail: "pop3 PASS not accepted"}
	}

	for _, cmd := range cfg.Commands {
		resp, err := pop3Cmd(tp, cmd.Command)
		if err != nil {
			return Result{OK: false, Detail: fmt.Sprintf("pop3 command error: %v", err)}
		}
		if cmd.Regex {
			matched, err := regexp.MatchString(cmd.Expected, resp)
			if err != nil {
				return Result{OK: false, Detail: fmt.Sprintf("pop3 regex error: %v", err)}
			}
			if !matched {
				return Result{OK: false, Detail: fmt.Sprintf("pop3 command %s did not match regex", cmd.Command)}
			}
		} else {
			if !strings.Contains(resp, cmd.Expected) {
				return Result{OK: false, Detail: fmt.Sprintf("pop3 command %s missing expected text", cmd.Command)}
			}
		}
	}

	return Result{OK: true, Detail: "POP3 login and commands passed"}
}

func pop3Cmd(tp *textproto.Conn, cmd string) (string, error) {
	id, err := tp.Cmd(cmd)
	if err != nil {
		return "", err
	}
	tp.StartResponse(id)
	defer tp.EndResponse(id)
	line, err := tp.ReadLine()
	return line, err
}

func checkFTP(ctx context.Context, cfg FTPCheck) Result {
	port := cfg.Port
	if port == 0 {
		port = 21
	}

	address := net.JoinHostPort(cfg.Host, strconv.Itoa(port))
	conn, err := ftp.Dial(address, ftp.DialWithContext(ctx), ftp.DialWithTimeout(deadlineFromCtx(ctx)))
	if err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("ftp dial error: %v", err)}
	}
	defer conn.Quit()

	if err := conn.Login(cfg.Username, cfg.Password); err != nil {
		return Result{OK: false, Detail: fmt.Sprintf("ftp login error: %v", err)}
	}

	for _, file := range cfg.Files {
		r, err := conn.Retr(file.Name)
		if err != nil {
			return Result{OK: false, Detail: fmt.Sprintf("ftp retrieve %s error: %v", file.Name, err)}
		}
		data, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			return Result{OK: false, Detail: fmt.Sprintf("ftp read %s error: %v", file.Name, err)}
		}

		if file.SHA256 != "" {
			sum := sha256.Sum256(data)
			hexSum := hex.EncodeToString(sum[:])
			if !strings.EqualFold(hexSum, file.SHA256) {
				return Result{OK: false, Detail: fmt.Sprintf("%s sha256 mismatch", file.Name)}
			}
		}

		if file.Regex != "" {
			matched, err := regexp.Match(file.Regex, data)
			if err != nil {
				return Result{OK: false, Detail: fmt.Sprintf("%s regex error: %v", file.Name, err)}
			}
			if !matched {
				return Result{OK: false, Detail: fmt.Sprintf("%s content regex mismatch", file.Name)}
			}
		}
	}

	return Result{OK: true, Detail: "FTP authentication and file checks passed"}
}

func checkDNS(ctx context.Context, cfg DNSCheck) Result {
	port := cfg.Port
	if port == 0 {
		port = 53
	}
	address := net.JoinHostPort(cfg.Server, strconv.Itoa(port))

	client := dns.Client{
		Net:     "udp",
		Timeout: deadlineFromCtx(ctx),
	}

	for _, record := range cfg.Records {
		qtype := dns.TypeA
		switch strings.ToUpper(record.Kind) {
		case "A":
			qtype = dns.TypeA
		case "MX":
			qtype = dns.TypeMX
		default:
			return Result{OK: false, Detail: fmt.Sprintf("unsupported DNS record kind: %s", record.Kind)}
		}

		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn(record.Domain), qtype)

		resp, _, err := client.ExchangeContext(ctx, msg, address)
		if err != nil {
			return Result{OK: false, Detail: fmt.Sprintf("dns query error: %v", err)}
		}
		if resp.Rcode != dns.RcodeSuccess {
			return Result{OK: false, Detail: fmt.Sprintf("dns query failed with rcode %d", resp.Rcode)}
		}

		switch qtype {
		case dns.TypeA:
			if !matchA(resp, record.Answers) {
				return Result{OK: false, Detail: fmt.Sprintf("A record mismatch for %s", record.Domain)}
			}
		case dns.TypeMX:
			if !matchMX(resp, record.Answers) {
				return Result{OK: false, Detail: fmt.Sprintf("MX record mismatch for %s", record.Domain)}
			}
		}
	}

	return Result{OK: true, Detail: "DNS resolution passed"}
}

func matchA(resp *dns.Msg, expected []string) bool {
	found := make(map[string]struct{})
	for _, ans := range resp.Answer {
		if a, ok := ans.(*dns.A); ok {
			found[a.A.String()] = struct{}{}
		}
	}
	for _, exp := range expected {
		if _, ok := found[exp]; ok {
			return true
		}
	}
	return false
}

func matchMX(resp *dns.Msg, expected []string) bool {
	found := make(map[string]struct{})
	for _, ans := range resp.Answer {
		if mx, ok := ans.(*dns.MX); ok {
			host := strings.TrimSuffix(mx.Mx, ".")
			found[host] = struct{}{}
		}
	}
	for _, exp := range expected {
		if _, ok := found[exp]; ok {
			return true
		}
	}
	return false
}

func deadlineFromCtx(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			return remaining
		}
		return time.Second
	}
	return 5 * time.Second
}

func errorsIsClosed(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "closed")
}
