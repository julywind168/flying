mod message;
mod node;
mod service;
mod utils;
mod flying;

use std::time::Duration;
use tokio::time::sleep;

use crate::node::Node;

#[tokio::main]
async fn main() {
    println!("Hello, world!");

    let node = Node::new("node1".to_string());
    node.spawn("main".to_string(), "service/main.lua".to_string()).await;

    loop {
        sleep(Duration::from_secs(1)).await;
    }
}
