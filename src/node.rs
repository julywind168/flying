use actix::Addr;
use mlua::prelude::*;
use std::{
    collections::HashMap,
    sync::{Arc, RwLock},
    time::Instant,
};

use crate::{
    service::{GetNextTickTime, LuaService, Tick},
    utils,
};

pub struct Node {
    _lua: Lua,
    env: LuaTable,
    pub start_time: u64,
    pub start_instant: std::time::Instant,
    pub services: Arc<RwLock<HashMap<String, Addr<LuaService>>>>,
}

impl Node {
    pub fn new() -> Self {
        let _lua = Lua::new();
        let env = _lua.create_table().unwrap();
        Node {
            _lua,
            env,
            start_time: utils::get_timestamp_ms(),
            start_instant: Instant::now(),
            services: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    pub fn gettime(&self) -> u64 {
        self.start_time + self.start_instant.elapsed().as_millis() as u64
    }

    pub fn setenv(&self, key: &str, value: String) {
        self.env.set(key, value).unwrap();
    }

    pub fn getenv(&self, key: &str) -> String {
        match self.env.get(key) {
            Ok(value) => value,
            Err(_) => "".to_string(),
        }
    }

    pub fn insert_service(&mut self, name: String, service: Addr<LuaService>) {
        self.services.write().unwrap().insert(name, service);
    }

    pub fn remove_service(&mut self, name: &str) {
        self.services.write().unwrap().remove(name);
    }
}

pub fn start_timer_thread(node: Arc<RwLock<Node>>) {
    let start_time = node.read().unwrap().start_time;
    let start_instant = node.read().unwrap().start_instant;
    let gettime = move || start_time + start_instant.elapsed().as_millis() as u64;
    let services = node.read().unwrap().services.clone();
    tokio::spawn(async move {
        print!("Timer thread started\n");
        loop {
            let services = services.read().unwrap().clone();
            let now = gettime();
            for (_, service) in services.iter() {
                let t = service.send(GetNextTickTime).await.unwrap();
                match t {
                    Some(t) => {
                        if t <= now {
                            service.do_send(Tick);
                        }
                    }
                    None => {}
                }
            }
            tokio::time::sleep(tokio::time::Duration::from_millis(10)).await;
        }
    });
}
