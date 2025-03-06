mod flying;
mod message;
mod node;
mod service;
mod utils;

use anyhow::Result;
use std::time::Duration;
use tokio::time::sleep;
use tracing_subscriber;
use tracing_subscriber::fmt;

use crate::node::Node;

#[tokio::main]
async fn main() -> Result<()> {
    let timer = fmt::time::ChronoLocal::new("%Y-%m-%d %H:%M:%S".to_string());
    tracing_subscriber::fmt()
        .with_timer(timer)
        .with_target(false)
        .with_max_level(tracing::Level::INFO)
        .init();

    let node = Node::new("node1".to_string());
    node.spawn("main".to_string(), "service/main.lua".to_string())
        .await?;

    loop {
        sleep(Duration::from_secs(1)).await;
    }
}
