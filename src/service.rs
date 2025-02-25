use std::{
    fs,
    sync::{Arc, RwLock},
};

use actix::prelude::*;
use mlua::prelude::*;

use crate::node::Node;

#[derive(Message)]
#[rtype(result = "()")]
struct Stop;

#[derive(Message)]
#[rtype(result = "String")]
pub struct Call(pub String);

pub struct LuaService {
    node: Arc<RwLock<Node>>,
    addr: Addr<Self>,
    lua: Lua,
    name: String,
    filename: String,
    version: u32, // reload or flow times
    serv: Option<LuaTable>,
}

impl LuaService {
    pub fn load(&mut self) {
        let script = fs::read_to_string(&self.filename).unwrap();
        let loaded = self.lua.load(&script).eval::<LuaTable>();
        match loaded {
            Ok(serv) => {
                let addr = self.addr.clone();
                let stop = self
                    .lua
                    .create_function(move |_, ()| {
                        addr.do_send(Stop);
                        Ok(())
                    })
                    .unwrap();
                serv.set("stop", stop).unwrap();
                self.serv = Some(serv);
            }
            Err(e) => panic!("service {} load error: {}", self.filename, e),
        }
    }
}

impl Actor for LuaService {
    type Context = Context<Self>;

    fn started(&mut self, _ctx: &mut Self::Context) {
        self.load();
        let f: LuaFunction = self.serv.as_ref().unwrap().get("_started").unwrap();
        let _ = f.call::<()>(self.version);
    }

    fn stopping(&mut self, _ctx: &mut Self::Context) -> Running {
        self.node.write().unwrap().remove_service(&self.name);
        let f: LuaFunction = self.serv.as_ref().unwrap().get("_stopping").unwrap();
        let _ = f.call::<()>(());
        Running::Stop
    }

    fn stopped(&mut self, _ctx: &mut Self::Context) {
        let f: LuaFunction = self.serv.as_ref().unwrap().get("_stopped").unwrap();
        let _ = f.call::<()>(());
    }
}

impl Handler<Call> for LuaService {
    type Result = String;

    fn handle(&mut self, msg: Call, _ctx: &mut Self::Context) -> Self::Result {
        let f: LuaFunction = self.serv.as_ref().unwrap().get("_message").unwrap();
        f.call::<String>(msg.0).unwrap()
    }
}

impl Handler<Stop> for LuaService {
    type Result = ();

    fn handle(&mut self, _: Stop, ctx: &mut Context<Self>) {
        ctx.stop();
    }
}

pub fn new(name: String, filename: String, node: Arc<RwLock<Node>>, version: u32) {
    let node_ = node.clone();
    let name_ = name.clone();
    let serv = LuaService::create(|ctx| {
        let lua = Lua::new();
        let flying: LuaTable = lua.load(r#"require "flying""#).eval().unwrap();

        let newservice = {
            let node = node.clone();
            lua.create_function(move |_, (name, filename)| {
                let _ = new(name, filename, node.clone(), 0);
                Ok(())
            })
            .unwrap()
        };

        let setenv = {
            let node = node.clone();
            lua.create_function(move |_, (key, value): (String, String)| {
                node.read().unwrap().setenv(&key, value);
                Ok(())
            })
            .unwrap()
        };

        let getenv = {
            let node = node.clone();
            lua.create_function(move |_, key: String| Ok(node.read().unwrap().getenv(&key)))
                .unwrap()
        };

        flying.set("newservice", newservice).unwrap();
        flying.set("setenv", setenv).unwrap();
        flying.set("getenv", getenv).unwrap();

        LuaService {
            node,
            addr: ctx.address(),
            lua,
            name,
            filename,
            version: version + 1,
            serv: None,
        }
    });
    node_.write().unwrap().insert_service(name_, serv);
}
