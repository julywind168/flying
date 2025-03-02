use std::{collections::HashMap, sync::Arc};
use tokio::sync::Mutex;

use crate::{
    message::Message,
    service::{self, Service}, utils,
};

pub struct Node {
    pub name: String,
    pub start_time: u64,
    start_instant: std::time::Instant,
    env: Mutex<HashMap<String, String>>,
    services: Mutex<HashMap<String, Service>>,
}

impl Node {
    pub fn new(name: String) -> Arc<Self> {
        Arc::new(Self {
            name,
            start_time: utils::get_timestamp_ms(),
            start_instant: std::time::Instant::now(),
            env: Mutex::new(HashMap::new()),
            services: Mutex::new(HashMap::new()),
        })
    }

    pub fn now(self: &Arc<Self>) -> u64 {
        self.start_instant.elapsed().as_millis() as u64
    }

    pub fn time(self: &Arc<Self>) -> u64 {
        self.start_time + self.now()
    }

    pub async fn setenv(self: &Arc<Self>, key: String, value: String) {
        self.env.lock().await.insert(key, value);
    }

    pub async fn getenv(self: &Arc<Self>, key: String) -> Option<String> {
        self.env.lock().await.get(&key).cloned()
    }

    pub async fn sendto(self: &Arc<Self>, dest: &str, msg: Message) {
        if let Some(service) = self.services.lock().await.get(dest) {
            service.send(msg).await.unwrap();
        } else {
            println!("Service {} not found", dest);
        }
    }

    pub async fn query(self: &Arc<Self>, name: &str) -> Option<Service> {
        self.services.lock().await.get(name).cloned()
    }

    pub async fn remove(self: &Arc<Self>, name: &str) {
        self.services.lock().await.remove(name);
    }

    pub async fn spawn(self: &Arc<Self>, name: String, scriptname: String) {
        self.services
            .lock()
            .await
            .insert(name.clone(), service::new(name, scriptname, self.clone()).await);
    }
}
