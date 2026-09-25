//! Detects hosts that expose more TCP ports than their assigned services need
//! (e.g. no firewall rules). Used by the "Online" panel of the dashboard.

use std::{collections::BTreeSet, time::Duration};

use tokio::{net::TcpStream, task::JoinSet, time::timeout};

use super::Team;

/// Ports probed on every participant, besides the ones they are expected to have.
/// Covers the usual Windows and Linux services that show up when there is no firewall.
pub const COMMON_PORTS: &[u16] = &[
    21, 22, 23, 25, 53, 80, 88, 110, 111, 135, 139, 143, 389, 443, 445, 464, 587, 636, 993,
    995, 1433, 2049, 3268, 3306, 3389, 5357, 5432, 5900, 5985, 5986, 8000, 8080, 8443,
];

/// Extra ports a participant is allowed to expose, comma or space separated
/// (e.g. `ALLOWED_PORTS: "3389,8080"`).
pub const ALLOWED_PORTS_VAR: &str = "ALLOWED_PORTS";

const CONNECT_TIMEOUT: Duration = Duration::from_millis(700);

/// Ports a domain controller legitimately needs open.
const AD_PORTS: &[u16] = &[53, 88, 135, 139, 389, 445, 464, 636, 3268, 3269];

fn env_port(team: &Team, var: &str) -> Option<u16> {
    team.env
        .iter()
        .find(|(k, _)| k == var)
        .and_then(|(_, v)| v.trim().parse().ok())
}

/// Port of a URL like `http://host:8080/path`, if it has an explicit one.
fn url_port(url: &str) -> Option<u16> {
    let rest = url.split_once("://").map_or(url, |(_, rest)| rest);
    let authority = rest.split('/').next()?;
    let (host, port) = authority.rsplit_once(':')?;
    if host.ends_with(']') || !host.contains(':') {
        port.parse().ok()
    } else {
        None
    }
}

fn env_url_port(team: &Team, var: &str) -> Option<u16> {
    team.env
        .iter()
        .find(|(k, _)| k == var)
        .and_then(|(_, v)| url_port(v))
}

/// Ports that should be open for this participant: the ports of their
/// assigned services plus anything listed in `ALLOWED_PORTS`.
pub fn expected_ports(team: &Team, services: &[String]) -> BTreeSet<u16> {
    let mut ports = BTreeSet::new();
    for service in services.iter().filter(|s| team.has_service(s)) {
        match service.to_ascii_uppercase().as_str() {
            "AD" => ports.extend(AD_PORTS),
            "DNS" => {
                ports.insert(53);
            }
            "FTP" => {
                ports.insert(env_port(team, "FTP_PORT").unwrap_or(21));
            }
            "WEB" => {
                ports.insert(env_url_port(team, "WEB_URL").unwrap_or(80));
            }
            "HTTPS" => {
                ports.insert(env_url_port(team, "HTTPS_URL").unwrap_or(443));
            }
            "SSH" => {
                ports.insert(env_port(team, "SSH_PORT").unwrap_or(22));
            }
            "SMTP" => {
                ports.insert(env_port(team, "SMTP_PORT").unwrap_or(25));
            }
            "IMAP" => {
                ports.insert(env_port(team, "IMAP_PORT").unwrap_or(143));
            }
            "POP3" => {
                ports.insert(env_port(team, "POP3_PORT").unwrap_or(110));
            }
            _ => {}
        }
    }
    if let Some((_, allowed)) = team.env.iter().find(|(k, _)| k == ALLOWED_PORTS_VAR) {
        ports.extend(
            allowed
                .split(|c: char| c == ',' || c.is_whitespace())
                .filter_map(|p| p.parse::<u16>().ok()),
        );
    }
    ports
}

/// TCP connect scan of `ports` on `ip`. Returns the open ones, sorted.
pub async fn scan_ports(ip: &str, ports: impl IntoIterator<Item = u16>) -> Vec<u16> {
    let mut tasks = JoinSet::new();
    for port in ports {
        let addr = format!("{}:{}", ip, port);
        tasks.spawn(async move {
            matches!(
                timeout(CONNECT_TIMEOUT, TcpStream::connect(addr)).await,
                Ok(Ok(_))
            )
            .then_some(port)
        });
    }
    let mut open = Vec::new();
    while let Some(res) = tasks.join_next().await {
        if let Ok(Some(port)) = res {
            open.push(port);
        }
    }
    open.sort_unstable();
    open
}

#[cfg(test)]
mod tests {
    use super::*;

    fn team_with_env(env: Vec<(&str, &str)>) -> Team {
        let mut team = Team::from_services(&vec![]);
        team.env = env
            .into_iter()
            .map(|(k, v)| (k.to_string(), v.to_string()))
            .collect();
        team
    }

    fn all_services() -> Vec<String> {
        ["AD", "DNS", "FTP", "WEB", "HTTPS", "SSH", "SMTP", "IMAP", "POP3"]
            .map(String::from)
            .to_vec()
    }

    #[test]
    fn expected_ports_follow_assigned_services() {
        let team = team_with_env(vec![("SERVICES", "FTP,WEB,HTTPS,DNS")]);
        let ports: Vec<u16> = expected_ports(&team, &all_services()).into_iter().collect();
        assert_eq!(ports, vec![21, 53, 80, 443]);
    }

    #[test]
    fn expected_ports_respect_overrides_and_allowed() {
        let team = team_with_env(vec![
            ("SERVICES", "SSH,WEB"),
            ("SSH_PORT", "2222"),
            ("WEB_URL", "http://10.0.0.5:8080/index.html"),
            ("ALLOWED_PORTS", "3389, 5985"),
        ]);
        let ports: Vec<u16> = expected_ports(&team, &all_services()).into_iter().collect();
        assert_eq!(ports, vec![2222, 3389, 5985, 8080]);
    }

    #[test]
    fn url_port_parsing() {
        assert_eq!(url_port("http://10.0.0.5"), None);
        assert_eq!(url_port("https://10.0.0.5:8443/x"), Some(8443));
        assert_eq!(url_port("http://[::1]:81/"), Some(81));
        assert_eq!(url_port("http://[::1]/"), None);
    }

    #[tokio::test]
    async fn scan_finds_listening_port() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let port = listener.local_addr().unwrap().port();
        assert_eq!(scan_ports("127.0.0.1", [port]).await, vec![port]);
    }
}
