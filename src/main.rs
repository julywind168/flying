use std::sync::{Arc, RwLock};

mod node;
mod service;
mod utils;

#[actix::main]
async fn main() {
    let node = Arc::new(RwLock::new(node::Node::new()));
    node::start_timer_thread(node.clone());
    service::new("main".to_string(), "service/main.lua".to_string(), node, 0);
    loop {
        tokio::time::sleep(tokio::time::Duration::from_secs(1)).await;
    }
}
