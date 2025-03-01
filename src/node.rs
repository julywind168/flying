use std::{collections::HashMap, sync::Arc};
use tokio::sync::Mutex;

use crate::{
    message::Message,
    service::{self, Service},
};

pub struct Node {
    pub name: String,
    env: Mutex<HashMap<String, String>>,
    services: Mutex<HashMap<String, Service>>,
}

impl Node {
    pub fn new(name: String) -> Arc<Self> {
        Arc::new(Self {
            name,
            env: Mutex::new(HashMap::new()),
            services: Mutex::new(HashMap::new()),
        })
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
