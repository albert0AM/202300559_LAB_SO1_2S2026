use axum::{extract::State, http::StatusCode, routing::{get, post}, Json, Router};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::sync::Arc;

#[derive(Deserialize, Serialize)]
struct Report {
    country: String,
    warplanes_in_air: i32,
    warships_in_water: i32,
    timestamp: String,
}

struct AppState {
    client: reqwest::Client,
    target: String,
}

async fn health() -> &'static str {
    "ok"
}

async fn handle(
    State(st): State<Arc<AppState>>,
    Json(r): Json<Report>,
) -> (StatusCode, Json<Value>) {
    if r.country.len() != 3 || r.warplanes_in_air < 0 || r.warships_in_water < 0 {
        return (StatusCode::BAD_REQUEST, Json(json!({"status": "invalid"})));
    }
    match st.client.post(&st.target).json(&r).send().await {
        Ok(resp) if resp.status().is_success() => {
            (StatusCode::OK, Json(json!({"status": "ok"})))
        }
        Ok(resp) => {
            eprintln!("go-api respondio {}", resp.status());
            (StatusCode::BAD_GATEWAY, Json(json!({"status": "upstream_error"})))
        }
        Err(e) => {
            eprintln!("error enviando a go-api: {e}");
            (StatusCode::BAD_GATEWAY, Json(json!({"status": "upstream_unreachable"})))
        }
    }
}

#[tokio::main]
async fn main() {
    let target = std::env::var("GO_API_URL")
        .unwrap_or_else(|_| "http://go-api-svc:8080/report".to_string());
    let state = Arc::new(AppState { client: reqwest::Client::new(), target });

    let app = Router::new()
        .route("/health", get(health))
        .route("/grpc-202300559", post(handle))
        .route("/report", post(handle))
        .with_state(state);

    let listener = tokio::net::TcpListener::bind("0.0.0.0:8000").await.unwrap();
    println!("rust-api escuchando en :8000");
    axum::serve(listener, app).await.unwrap();
}
