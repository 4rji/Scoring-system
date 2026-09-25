mod admin;
mod team;

use axum::{
    extract::State, http::StatusCode, routing::{get, post}, Json, Router
};
use serde::{Deserialize, Serialize};
use tokio::{process::Command, task::JoinSet, time::timeout};

use crate::{
    auth::{Auth, TeamCredentials},
    checker::{exposure, ScoreboardInfo},
};

use axum_login::{
    tower_sessions::{MemoryStore, SessionManagerLayer},
    AuthManagerLayerBuilder,
};

use crate::ConfigState;
use std::{
    collections::HashMap,
    sync::OnceLock,
    time::{Duration, Instant, SystemTime, UNIX_EPOCH},
};
use tokio::sync::Mutex;

pub type AuthSession = axum_login::AuthSession<Auth>;

pub fn main_router(state: ConfigState) -> Router<ConfigState> {
    let session_store = MemoryStore::default();
    let session_layer = SessionManagerLayer::new(session_store);

    let backend = Auth::new(&state);
    let auth_layer = AuthManagerLayerBuilder::new(backend, session_layer).build();

    Router::new()
        .nest("/admin", admin::admin_router())
        .nest("/team", team::team_router(state))
        .route("/scores", get(scores))
        .route("/time", get(time))
        .route("/competition", get(competition_status))
        .route("/competition/injects", get(competition_injects))
        .route("/competition/start", post(start_competition))
        .route("/login", post(login))
        .route("/info", get(scoreboard_info))
        .route("/reachability", get(reachability))
        .layer(auth_layer)
}

#[derive(Serialize)]
struct ScoreWrapper {
    teams: Vec<ScoreBody>,
    services: Vec<String>,
}

#[derive(Serialize)]
struct ScoreBody {
    name: String,
    score: u32,
    /// Only the services assigned to this participant.
    services: Vec<ServiceStatus>,
}

#[derive(Serialize)]
struct ServiceStatus {
    name: String,
    up: bool,
    /// Whether the service has been checked at least once.
    checked: bool,
    uptime: f32,
    history: Vec<bool>,
}

#[derive(Serialize)]
struct TimeBody {
    minutes: u64,
    seconds: u64,
    active: bool,
}

#[derive(Serialize)]
struct ReachabilityStatus {
    name: String,
    ip: String,
    method: String,
    reachable: bool,
    /// Open TCP ports found by the scan.
    open_ports: Vec<u16>,
    /// Open ports that none of the participant's services need.
    extra_ports: Vec<u16>,
}

#[derive(Serialize)]
struct CompetitionStatus {
    started_at_ms: Option<u64>,
}

#[derive(Serialize)]
struct CompetitionInject {
    name: String,
    start: u32,
    duration: u32,
}

async fn time(State(state): State<ConfigState>) -> Json<TimeBody> {
    let config = state.read().await;
    let runtime = config.run_time();
    Json(TimeBody {
        minutes: runtime.as_secs() / 60,
        seconds: runtime.as_secs() % 60,
        active: config.is_active(),
    })
}

async fn competition_status(State(state): State<ConfigState>) -> Json<CompetitionStatus> {
    let config = state.read().await;
    Json(CompetitionStatus {
        started_at_ms: config.competition_start_ms(),
    })
}

async fn competition_injects(State(state): State<ConfigState>) -> Json<Vec<CompetitionInject>> {
    let config = state.read().await;
    let injects = config
        .injects
        .iter()
        .map(|inject| CompetitionInject {
            name: inject.name.clone(),
            start: inject.start,
            duration: inject.duration,
        })
        .collect();
    Json(injects)
}

async fn start_competition(State(state): State<ConfigState>) -> Json<CompetitionStatus> {
    let mut config = state.write().await;
    let start_ms = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_else(|_| Duration::from_secs(0))
        .as_millis() as u64;
    let stored = config.set_competition_start_ms(start_ms);
    Json(CompetitionStatus {
        started_at_ms: Some(stored),
    })
}

async fn scoreboard_info() -> Json<ScoreboardInfo> {
    Json(ScoreboardInfo::default())
}

async fn scores(State(state): State<ConfigState>) -> Json<ScoreWrapper> {
    let config = state.read().await;
    let services = config.services.iter().map(|s| s.name.clone());
    let scores = config.teams.iter().map(|(name, team)| ScoreBody {
        name: name.to_owned(),
        score: team.score(),
        services: config
            .services
            .iter()
            .filter(|s| team.has_service(&s.name))
            .map(|s| {
                let score = team.scores.get(&s.name).cloned().unwrap_or_default();
                ServiceStatus {
                    name: s.name.clone(),
                    up: score.up,
                    checked: score.checks > 0,
                    uptime: score.uptime(),
                    history: score.history.into_iter().collect(),
                }
            })
            .collect(),
    });
    Json(ScoreWrapper {
        teams: scores.collect(),
        services: services.collect(),
    })
}

#[derive(Deserialize)]
struct LoginPayload {
    username: String,
    password: String,
}

async fn login(
    mut auth: AuthSession,
    Json(payload): Json<LoginPayload>,
) -> Result<StatusCode, StatusCode> {
    let creds = TeamCredentials {
        name: payload.username,
        password: payload.password,
    };
    if let Ok(Some(user)) = auth.authenticate(creds).await {
        auth.login(&user).await.map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;
        Ok(StatusCode::OK)
    } else {
        Err(StatusCode::UNAUTHORIZED)
    }
}

async fn probe_host(ip: &str, _port: u16) -> bool {
    // Fallback probe using a single ICMP echo (ping). This assumes the system ping binary is available.
    let mut cmd = Command::new("ping");
    cmd.arg("-c").arg("1").arg("-W").arg("1").arg(ip)
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null());
    match timeout(Duration::from_secs(2), cmd.status()).await {
        Ok(Ok(status)) => status.success(),
        _ => false,
    }
}

/// How long a port scan of a host is reused before scanning it again.
const PORT_SCAN_TTL: Duration = Duration::from_secs(30);

fn port_scan_cache() -> &'static Mutex<HashMap<String, (Instant, Vec<u16>)>> {
    static CACHE: OnceLock<Mutex<HashMap<String, (Instant, Vec<u16>)>>> = OnceLock::new();
    CACHE.get_or_init(Default::default)
}

async fn cached_scan(ip: &str, ports: Vec<u16>) -> Vec<u16> {
    if let Some((at, open)) = port_scan_cache().lock().await.get(ip) {
        if at.elapsed() < PORT_SCAN_TTL {
            return open.clone();
        }
    }
    let open = exposure::scan_ports(ip, ports).await;
    port_scan_cache()
        .lock()
        .await
        .insert(ip.to_string(), (Instant::now(), open.clone()));
    open
}

/// Pings the `IP` env var of every participant that has one and scans it for
/// open ports that their assigned services don't need.
async fn reachability(State(state): State<ConfigState>) -> Json<Vec<ReachabilityStatus>> {
    let targets: Vec<(String, String, Vec<u16>)> = {
        let config = state.read().await;
        let services: Vec<String> = config.services.iter().map(|s| s.name.clone()).collect();
        config
            .teams
            .iter()
            .filter_map(|(name, team)| {
                team.env
                    .iter()
                    .find(|(k, v)| k == "IP" && !v.is_empty())
                    .map(|(_, ip)| {
                        let expected = exposure::expected_ports(team, &services);
                        (name.clone(), ip.clone(), expected.into_iter().collect())
                    })
            })
            .collect()
    };

    let mut tasks = JoinSet::new();
    for (name, ip, expected) in targets {
        tasks.spawn(async move {
            let mut to_scan: Vec<u16> = exposure::COMMON_PORTS.to_vec();
            to_scan.extend(&expected);
            to_scan.sort_unstable();
            to_scan.dedup();
            let (pinged, open_ports) =
                tokio::join!(probe_host(&ip, 0), cached_scan(&ip, to_scan));
            let extra_ports = open_ports
                .iter()
                .copied()
                .filter(|p| !expected.contains(p))
                .collect();
            ReachabilityStatus {
                name,
                ip,
                method: "ICMP ping + TCP port scan".to_string(),
                // Hosts that drop ICMP still count as online if a port answers.
                reachable: pinged || !open_ports.is_empty(),
                open_ports,
                extra_ports,
            }
        });
    }

    let mut results = Vec::new();
    while let Some(res) = tasks.join_next().await {
        if let Ok(status) = res {
            results.push(status);
        }
    }

    results.sort_by(|a, b| a.name.cmp(&b.name));

    Json(results)
}
