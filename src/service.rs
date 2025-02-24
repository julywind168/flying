use std::fs;

use actix::prelude::*;
use mlua::prelude::*;

#[derive(Message)]
#[rtype(result = "()")]
struct Stop;

#[derive(Message)]
#[rtype(result = "String")]
pub struct Call(pub String);

pub struct LuaService {
    addr: Addr<Self>,
    lua: Lua,
    file: String,
    version: u32, // reload or flow times
    serv: Option<LuaTable>,
}

impl LuaService {
    pub fn load(&mut self) {
        let script = fs::read_to_string(&self.file).unwrap();
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
            Err(e) => panic!("service {} load error: {}", self.file, e),
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

pub fn new(file: &str, version: u32) -> Addr<LuaService> {
    LuaService::create(|ctx| {
        let lua: Lua = Lua::new();
        let addr = ctx.address();
        LuaService {
            addr,
            lua,
            file: file.to_string(),
            version: version + 1,
            serv: None,
        }
    })
}