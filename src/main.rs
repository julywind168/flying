use std::sync::Arc;

mod node;
mod service;

#[actix::main]
async fn main() {
    let node = Arc::new(node::Node::new());
    let _ = service::new("service/main.lua".to_string(), node, 0);
}
