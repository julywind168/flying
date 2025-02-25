use std::sync::{Arc, RwLock};

mod node;
mod service;

#[actix::main]
async fn main() {
    let node = Arc::new(RwLock::new(node::Node::new()));
    let _ = service::new("main".to_string(), "service/main.lua".to_string(), node, 0);
}
