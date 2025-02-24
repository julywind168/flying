use mlua::prelude::*;

pub struct Node {
    _lua: Lua,
    env: LuaTable,
}

impl Node {
    pub fn new() -> Self {
        let _lua = Lua::new();
        let env = _lua.globals();
        Node { _lua, env }
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
}