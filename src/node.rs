use dashmap::DashMap;
use std::sync::Arc;

use crate::{
    message::Message,
    service::{self, Service},
    utils,
};

pub struct Node {
    pub name: String,
    pub start_time: u64,
    start_instant: std::time::Instant,
    env: DashMap<String, String>,
    services: DashMap<String, Service>,
}

impl Node {
    pub fn new(name: String) -> Arc<Self> {
        Arc::new(Self {
            name,
            start_time: utils::get_timestamp_ms(),
            start_instant: std::time::Instant::now(),
            env: DashMap::new(),
            services: DashMap::new(),
        })
    }

    pub fn now(self: &Arc<Self>) -> u64 {
        self.start_instant.elapsed().as_millis() as u64
    }

    pub fn time(self: &Arc<Self>) -> u64 {
        self.start_time + self.now()
    }

    pub fn setenv(self: &Arc<Self>, key: String, value: String) {
        self.env.insert(key, value);
    }

    pub fn getenv(self: &Arc<Self>, key: &str) -> Option<String> {
        self.env.get(key).map(|v| v.clone())
    }

    pub async fn sendto(self: &Arc<Self>, dest: &str, msg: Message) {
        if let Some(service) = self.services.get(dest) {
            service.send(msg).await.unwrap();
        } else {
            println!("Service {} not found", dest);
        }
    }

    pub fn query(self: &Arc<Self>, name: &str) -> Option<Service> {
        self.services.get(name).map(|v| v.clone())
    }

    pub fn remove(self: &Arc<Self>, name: &str) {
        self.services.remove(name);
    }

    pub async fn spawn(self: &Arc<Self>, name: String, scriptname: String) {
        self.services.insert(
            name.clone(),
            service::new(name, scriptname, self.clone()).await,
        );
    }
}
