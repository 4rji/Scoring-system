import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  useCompetitionInjects,
  useCompetitionStatus,
  useReachability,
  useScore,
  useStartCompetition,
} from "../Hooks/CtrlHooks";
import { ReachabilityStatus, ServiceStatus } from "../types";
import { formatDate } from "../util";

const serviceDescriptions: Record<string, string> = {
  AD: "TCP 389: Anonymous LDAP query",
  DNS: "UDP/TCP 53: Resolve record via participant's DNS server",
  FTP: "TCP 21: FTP 220 banner",
  WEB: "TCP 80: HTTP GET answers below 500",
  HTTPS: "TCP 443: HTTPS GET answers below 500",
  SSH: "TCP 22: SSH banner",
  SMTP: "TCP 25: SMTP 220 banner",
  IMAP: "TCP 143: IMAP * OK banner",
  POP3: "TCP 110: POP3 +OK banner",
};

// 1-2 unexpected open ports is yellow, 3 or more (likely no firewall) is orange.
const reachabilityColor = (status: ReachabilityStatus) => {
  if (!status.reachable) return "border-red-700 bg-red-500";
  if (status.extra_ports.length >= 3) return "border-orange-700 bg-orange-500";
  if (status.extra_ports.length > 0) return "border-yellow-600 bg-yellow-400";
  return "border-blue-700 bg-blue-500";
};

const portNames: Record<number, string> = {
  21: "FTP",
  22: "SSH",
  23: "Telnet",
  25: "SMTP",
  53: "DNS",
  80: "HTTP",
  88: "Kerberos",
  110: "POP3",
  111: "RPCbind",
  135: "MSRPC",
  139: "NetBIOS",
  143: "IMAP",
  389: "LDAP",
  443: "HTTPS",
  445: "SMB",
  464: "Kpasswd",
  587: "Submission",
  636: "LDAPS",
  993: "IMAPS",
  995: "POP3S",
  1433: "MSSQL",
  2049: "NFS",
  3268: "Global Catalog",
  3269: "Global Catalog SSL",
  3306: "MySQL",
  3389: "RDP",
  5357: "WSDAPI",
  5432: "PostgreSQL",
  5900: "VNC",
  5985: "WinRM",
  5986: "WinRM HTTPS",
  8000: "HTTP alt",
  8080: "HTTP alt",
  8443: "HTTPS alt",
};

const PortList = ({
  title,
  ports,
  className,
}: {
  title: string;
  ports: number[];
  className: string;
}) => (
  <div>
    <div className="mb-1 text-xs font-bold uppercase text-zinc-400">
      {title}
    </div>
    <ul className="space-y-0.5 text-sm">
      {ports.map((port) => (
        <li className="flex justify-between gap-3" key={port}>
          <span className={"font-mono font-semibold " + className}>
            {port}/tcp
          </span>
          <span className="text-zinc-300">{portNames[port] ?? ""}</span>
        </li>
      ))}
    </ul>
  </div>
);

const PortPopover = ({ status }: { status: ReachabilityStatus }) => {
  const expected = status.open_ports.filter(
    (port) => !status.extra_ports.includes(port)
  );
  return (
    <div
      className="absolute left-1/2 top-full z-10 mt-2 w-48 -translate-x-1/2 space-y-2 rounded-lg border border-zinc-600 bg-zinc-900 p-3 text-left shadow-xl"
      onClick={(e) => e.stopPropagation()}
    >
      {status.open_ports.length === 0 && (
        <p className="text-sm text-zinc-400">No open ports found.</p>
      )}
      {status.extra_ports.length > 0 && (
        <PortList
          title="Unexpected"
          ports={status.extra_ports}
          className={
            status.extra_ports.length >= 3
              ? "text-orange-400"
              : "text-yellow-300"
          }
        />
      )}
      {expected.length > 0 && (
        <PortList title="Expected" ports={expected} className="text-blue-300" />
      )}
      <p className="text-[0.65rem] text-zinc-500">{status.method}</p>
    </div>
  );
};

const panelClass =
  "rounded-xl border border-zinc-700 bg-zinc-800 p-3 shadow-md";

const OnlinePanel = () => {
  const { reachability, reachabilityLoading, reachabilityError } =
    useReachability();
  const [selected, setSelected] = useState<string | null>(null);

  // Close the port list when clicking anywhere else.
  useEffect(() => {
    if (!selected) return;
    const close = () => setSelected(null);
    document.addEventListener("click", close);
    return () => document.removeEventListener("click", close);
  }, [selected]);

  const renderBody = () => {
    if (reachabilityLoading) return <p className="text-center">Loading...</p>;
    if (reachabilityError) return <p className="text-center">Error!</p>;
    if (reachability.length === 0)
      return (
        <p className="text-center text-sm text-zinc-400">
          No participant has an IP configured.
        </p>
      );
    return (
      <div className="flex flex-wrap justify-center gap-x-8 gap-y-3">
        {reachability.map((status) => (
          <div
            className="relative flex min-w-[5rem] cursor-pointer flex-col items-center"
            key={status.name}
            onClick={(e) => {
              e.stopPropagation();
              setSelected(selected === status.name ? null : status.name);
            }}
          >
            <div
              className={
                "flex h-11 w-11 items-center justify-center rounded-full border-4 text-xl font-black text-white shadow-md " +
                reachabilityColor(status)
              }
            >
              &bull;
            </div>
            <div className="mt-1 text-sm font-bold leading-none">
              {status.name}
            </div>
            <div className="mt-1 text-xs leading-none text-zinc-100">
              {status.ip}
            </div>
            {status.reachable && status.extra_ports.length > 0 && (
              <div
                className={
                  "mt-1 text-xs font-semibold leading-none " +
                  (status.extra_ports.length >= 3
                    ? "text-orange-400"
                    : "text-yellow-300")
                }
              >
                +{status.extra_ports.length} Open
              </div>
            )}
            {selected === status.name && <PortPopover status={status} />}
          </div>
        ))}
      </div>
    );
  };

  return (
    <section className={panelClass}>
      <h2 className="mb-2 text-center text-2xl font-extrabold">Online</h2>
      {renderBody()}
    </section>
  );
};

const Leaderboard = () => {
  const { data, scoreLoading, scoreError } = useScore();
  return (
    <section className={panelClass}>
      <h2 className="mb-2 text-center text-2xl font-extrabold">Leaderboard</h2>
      {scoreLoading || scoreError ? (
        <p className="text-center">{scoreLoading ? "Loading..." : "Error!"}</p>
      ) : (
        <table className="mx-auto table-auto">
          <tbody>
            {data.teams
              .concat()
              .sort((a, b) => b.score - a.score)
              .map((team, i) => (
                <tr className="text-xl" key={"Leader" + team.name}>
                  <td className="font-medium">
                    <Link to={"/team/" + team.name}>
                      {i + 1}. {team.name}:
                    </Link>
                  </td>
                  <td className="px-2">{team.score}</td>
                </tr>
              ))}
          </tbody>
        </table>
      )}
    </section>
  );
};

const ServiceChip = ({ service }: { service: ServiceStatus }) => {
  const color = !service.checked
    ? "border-zinc-500 bg-zinc-600"
    : service.up
    ? "border-green-700 bg-green-500"
    : "border-red-700 bg-red-500";
  const description = serviceDescriptions[service.name];
  const history = service.history.map((up) => (up ? "▲" : "▼")).join(" ");
  return (
    <div
      className={
        "flex min-w-[5.5rem] flex-col items-center rounded-lg border-2 px-3 py-1 text-white shadow-md " +
        color
      }
      title={[description, history && `Last checks (newest first): ${history}`]
        .filter(Boolean)
        .join("\n")}
    >
      <span className="text-lg font-extrabold leading-tight">
        {service.name}
      </span>
      <span className="text-xs font-semibold leading-tight">
        {service.checked ? `${service.uptime.toFixed(0)}% up` : "pending"}
      </span>
    </div>
  );
};

const averageUptime = (services: ServiceStatus[]) => {
  const checked = services.filter((s) => s.checked);
  if (checked.length === 0) return null;
  return checked.reduce((sum, s) => sum + s.uptime, 0) / checked.length;
};

const Scoreboard = () => {
  const { data, scoreLoading, scoreError, scoreUpdatedAt } = useScore();
  if (scoreLoading) return <div className="px-4 py-5">Loading...</div>;
  if (scoreError) return <div className="px-4 py-5">Error!</div>;
  return (
    <section className="px-4 py-5">
      <table className="w-full table-auto">
        <thead>
          <tr>
            <th className="w-44 px-2 py-2 text-left font-extrabold">
              Participant
            </th>
            <th className="px-2 py-2 text-left font-extrabold">Services</th>
            <th className="w-28 px-2 py-2 text-center font-extrabold">
              Uptime
            </th>
            <th className="w-24 px-2 py-2 text-center font-extrabold">
              Score
            </th>
          </tr>
        </thead>
        <tbody className="bg-zinc-800 shadow-md">
          {data.teams.map((team) => {
            const uptime = averageUptime(team.services);
            const upCount = team.services.filter((s) => s.up).length;
            return (
              <tr key={"ScoreRow" + team.name}>
                <td className="border border-zinc-700 px-2 text-xl font-bold">
                  <Link to={"/team/" + team.name}>{team.name}</Link>
                  <div className="text-xs font-normal text-zinc-300">
                    {upCount}/{team.services.length} up
                  </div>
                </td>
                <td className="border border-zinc-700 px-2 py-2">
                  <div className="flex flex-wrap gap-2">
                    {team.services.length === 0 && (
                      <span className="text-sm text-zinc-400">
                        No services assigned
                      </span>
                    )}
                    {team.services.map((service) => (
                      <ServiceChip
                        key={team.name + service.name}
                        service={service}
                      />
                    ))}
                  </div>
                </td>
                <td className="border border-zinc-700 px-2 text-center text-lg font-semibold">
                  {uptime === null ? "-" : `${uptime.toFixed(1)}%`}
                </td>
                <td className="border border-zinc-700 px-2 text-center text-lg font-semibold">
                  {team.score}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      <label className="text-sm">
        Last Updated: {formatDate(new Date(scoreUpdatedAt))}
      </label>
    </section>
  );
};

const formatClock = (minutes: number) => {
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  return `${hours.toString().padStart(2, "0")}:${mins
    .toString()
    .padStart(2, "0")}`;
};

const formatCountdown = (totalSeconds: number) => {
  const clamped = Math.max(0, totalSeconds);
  const minutes = Math.floor(clamped / 60);
  const seconds = clamped % 60;
  return `${minutes}:${seconds < 10 ? "0" : ""}${seconds}`;
};

const InjectSchedule = () => {
  const { competition, competitionLoading } = useCompetitionStatus();
  const { competitionInjects, competitionInjectsLoading } =
    useCompetitionInjects();
  const { startCompetition, startCompetitionLoading } = useStartCompetition();
  const [now, setNow] = useState(Date.now());
  const startedAt = competition?.started_at_ms || null;
  const elapsedSeconds = startedAt
    ? Math.floor((now - startedAt) / 1000)
    : 0;

  useEffect(() => {
    if (!startedAt) return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [startedAt]);

  const schedule = useMemo(
    () =>
      competitionInjects
        .map((inject) => ({
          ...inject,
          end: inject.start + inject.duration,
          startSeconds: inject.start * 60,
        }))
        .sort((a, b) => a.startSeconds - b.startSeconds),
    [competitionInjects]
  );

  const visibleInjects = startedAt
    ? schedule.filter((inject) => elapsedSeconds >= inject.startSeconds)
    : [];

  const renderTimeRemaining = (inject: (typeof schedule)[number]) => {
    const secondsUntilStart = inject.startSeconds - elapsedSeconds;
    if (secondsUntilStart > 0) {
      return `Starts in ${formatCountdown(secondsUntilStart)}`;
    }

    const secondsRemaining =
      inject.startSeconds + inject.duration * 60 - elapsedSeconds;
    if (secondsRemaining <= 0) return "Ended";
    return formatCountdown(secondsRemaining);
  };

  const renderRows = () => {
    if (competitionLoading || competitionInjectsLoading) {
      return (
        <tr>
          <td className="p-2 text-center" colSpan={5}>
            Loading...
          </td>
        </tr>
      );
    }

    if (!startedAt) {
      return (
        <tr>
          <td className="p-2 text-center" colSpan={5}>
            Click Start Competition to begin.
          </td>
        </tr>
      );
    }

    if (schedule.length === 0) {
      return (
        <tr>
          <td className="p-2 text-center" colSpan={5}>
            No injects configured.
          </td>
        </tr>
      );
    }

    if (visibleInjects.length === 0) {
      return (
        <tr>
          <td className="p-2 text-center" colSpan={5}>
            Waiting for the first inject...
          </td>
        </tr>
      );
    }

    return visibleInjects.map((inject) => (
      <tr className="border-t border-zinc-700" key={`${inject.name}-${inject.start}`}>
        <td className="p-2">{formatClock(inject.start)}</td>
        <td className="p-2">{formatClock(inject.end)}</td>
        <td className="p-2 font-medium">{inject.name}</td>
        <td className="p-2">{inject.duration}</td>
        <td className="p-2 font-mono">{renderTimeRemaining(inject)}</td>
      </tr>
    ));
  };

  return (
    <section className="mx-4 mb-5 rounded-xl border border-zinc-700 bg-zinc-800 p-2 shadow-md">
      <h2 className="mb-2 text-center text-3xl font-bold">Inject Schedule</h2>
      <div className="mb-3 flex justify-center">
        <button
          className="rounded bg-zinc-700 px-4 py-2 text-lg font-semibold text-zinc-100 shadow-sm hover:bg-zinc-600 disabled:cursor-not-allowed disabled:opacity-50"
          disabled={!!startedAt || startCompetitionLoading}
          onClick={() => startCompetition()}
        >
          {startedAt ? "Competition Started" : "Start Competition"}
        </button>
      </div>
      <table className="table-auto w-full">
        <thead>
          <tr className="text-left">
            <th className="p-2">Start</th>
            <th className="p-2">End</th>
            <th className="p-2">Inject</th>
            <th className="p-2">Duration</th>
            <th className="p-2">Time Remaining</th>
          </tr>
        </thead>
        <tbody>{renderRows()}</tbody>
      </table>
    </section>
  );
};

function App() {
  return (
    <div className="min-h-screen w-full bg-zinc-900 text-zinc-100">
      <header className="border-b border-zinc-700 bg-zinc-950 py-4">
        <h1 className="text-center text-6xl font-extrabold">
          Metro CCDC Individual Scoreboard {import.meta.env.DEV ? "(DEV)" : ""}
        </h1>
      </header>
      <div className="grid gap-4 px-4 pt-5 md:grid-cols-2">
        <OnlinePanel />
        <Leaderboard />
      </div>
      <Scoreboard />
      <InjectSchedule />
    </div>
  );
}

export default App;
