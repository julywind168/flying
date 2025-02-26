use std::{
    fs,
    sync::{Arc, RwLock},
};

use actix::prelude::*;
use mlua::prelude::*;

use crate::node::Node;

#[derive(Message)]
#[rtype(result = "()")]
pub struct SetNextTickTime(pub Option<u64>);

impl Handler<SetNextTickTime> for LuaService {
    type Result = ();

    fn handle(&mut self, msg: SetNextTickTime, _ctx: &mut Self::Context) {
        self.next_tick_time = msg.0;
    }
}

#[derive(Message)]
#[rtype(result = "Option<u64>")]
pub struct GetNextTickTime;

impl Handler<GetNextTickTime> for LuaService {
    type Result = Option<u64>;

    fn handle(&mut self, _: GetNextTickTime, _ctx: &mut Self::Context) -> Self::Result {
        self.next_tick_time
    }
}

#[derive(Message)]
#[rtype(result = "()")]
pub struct Tick;

impl Handler<Tick> for LuaService {
    type Result = ();

    fn handle(&mut self, _: Tick, _ctx: &mut Self::Context) {
        let f: LuaFunction = self.serv.as_ref().unwrap().get("_tick").unwrap();
        let _ = f.call::<()>(());
    }
}

#[repr(u8)]
pub enum LuaMessageType {
    Request,
    Response,
}

impl From<u8> for LuaMessageType {
    fn from(value: u8) -> Self {
        match value {
            0 => LuaMessageType::Request,
            1 => LuaMessageType::Response,
            _ => panic!("Invalid LuaMessageType value {}", value),
        }
    }
}

#[derive(Message)]
#[rtype(result = "()")]
pub struct LuaMessage {
    pub source: String,
    pub session: u32,
    pub ty: LuaMessageType,
    pub data: String,
}

pub struct LuaService {
    node: Arc<RwLock<Node>>,
    addr: Addr<Self>,
    lua: Lua,
    name: String,
    filename: String,
    version: u32, // reload or flow times
    serv: Option<LuaTable>,
    next_tick_time: Option<u64>,
}

impl Handler<LuaMessage> for LuaService {
    type Result = ();
    fn handle(&mut self, msg: LuaMessage, _ctx: &mut Self::Context) -> Self::Result {
        let f: LuaFunction = self.serv.as_ref().unwrap().get("_message").unwrap();
        f.call::<()>((msg.source, msg.session, msg.ty as u8, msg.data))
            .unwrap()
    }
}

#[derive(Message)]
#[rtype(result = "()")]
struct Stop;

impl Handler<Stop> for LuaService {
    type Result = ();
    fn handle(&mut self, _: Stop, ctx: &mut Context<Self>) {
        ctx.stop();
    }
}

impl LuaService {
    pub fn load(&mut self) {
        let script = fs::read_to_string(&self.filename).unwrap();
        let loaded = self.lua.load(&script).eval::<LuaTable>();
        match loaded {
            Ok(serv) => {
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

pub fn new(name: String, filename: String, node: Arc<RwLock<Node>>, version: u32) {
    let node_ = node.clone();
    let name_ = name.clone();
    let serv = LuaService::create(|ctx| {
        let lua = Lua::new();
        let flying: LuaTable = lua.load(r#"require "flying""#).eval().unwrap();

        let stop = {
            let addr = ctx.address();
            lua.create_function(move |_, ()| {
                addr.do_send(Stop);
                Ok(())
            })
            .unwrap()
        };

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

        let send_message = {
            let node = node.clone();
            let source = name.clone();
            lua.create_function(
                move |_, (dest, session, ty, data): (String, u32, u8, String)| {
                    let msg = LuaMessage {
                        source: source.clone(),
                        session,
                        ty: ty.into(),
                        data,
                    };
                    let serv = node.read().unwrap().services.read().unwrap().get(&dest).unwrap().clone();
                    serv.do_send(msg);
                    Ok(())
                },
            )
            .unwrap()
        };

        let starttime = {
            let node = node.clone();
            lua.create_function(move |_, ()| Ok(node.read().unwrap().start_time))
                .unwrap()
        };

        let now = {
            let node = node.clone();
            lua.create_function(move |_, ()| {
                Ok(node.read().unwrap().start_instant.elapsed().as_millis() as u64)
            })
            .unwrap()
        };

        let time = {
            let node = node.clone();
            lua.create_function(move |_, ()| {
                Ok(node.read().unwrap().start_time
                    + node.read().unwrap().start_instant.elapsed().as_millis() as u64)
            })
            .unwrap()
        };

        let set_next_tick_time = {
            let addr = ctx.address();
            lua.create_function(move |_, time| {
                addr.do_send(SetNextTickTime(time));
                Ok(())
            })
            .unwrap()
        };

        flying.set("name", name.clone()).unwrap();
        flying.set("stop", stop).unwrap();
        flying.set("newservice", newservice).unwrap();
        flying.set("setenv", setenv).unwrap();
        flying.set("getenv", getenv).unwrap();
        flying.set("send_message", send_message).unwrap();
        flying.set("starttime", starttime).unwrap();
        flying.set("now", now).unwrap();
        flying.set("time", time).unwrap();
        flying.set("set_next_tick_time", set_next_tick_time)
            .unwrap();

        LuaService {
            node,
            addr: ctx.address(),
            lua,
            name,
            filename,
            version: version + 1,
            serv: None,
            next_tick_time: None,
        }
    });
    node_.write().unwrap().insert_service(name_, serv);
}
