use std::{sync::{Arc, RwLock}, time::Duration};

use actix::clock::sleep;

mod node;
mod service;
mod utils;

#[actix::main]
async fn main() {
    let node = Arc::new(RwLock::new(node::Node::new()));
    let _ = service::new("main".to_string(), "service/main.lua".to_string(), node, 0);

    sleep(Duration::from_secs(3)).await;
}
