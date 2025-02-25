use actix::Addr;
use mlua::prelude::*;
use std::collections::HashMap;

use crate::service::LuaService;

pub struct Node {
    _lua: Lua,
    env: LuaTable,
    services: HashMap<String, Addr<LuaService>>,
}

impl Node {
    pub fn new() -> Self {
        let _lua = Lua::new();
        let env = _lua.globals();
        Node { _lua, env, services: HashMap::new() }
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
        self.services.insert(name, service);
    }

    pub fn remove_service(&mut self, name: &str) {
        self.services.remove(name);
    }
}