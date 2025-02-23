use actix::prelude::*;
use mlua::prelude::*;

#[derive(Message)]
#[rtype(result = "String")]
pub struct Call(pub String);

pub struct LuaService {
    addr : Option<Addr<Self>>,
    _lua: Lua,
    serv: LuaTable,
}

impl Actor for LuaService {
    type Context = Context<Self>;

    fn started(&mut self, _ctx: &mut Self::Context) {
        let f: LuaFunction = self.serv.get("_started").unwrap();
        let _ = f.call::<()>(());
    }

    fn stopping(&mut self, _ctx: &mut Self::Context) -> Running {
        let f: LuaFunction = self.serv.get("_stopping").unwrap();
        let _ = f.call::<()>(());
        Running::Stop
    }

    fn stopped(&mut self, _ctx: &mut Self::Context) {
        let f: LuaFunction = self.serv.get("_stopped").unwrap();
        let _ = f.call::<()>(());
    }
}

impl Handler<Call> for LuaService {
    type Result = String;

    fn handle(&mut self, msg: Call, _ctx: &mut Self::Context) -> Self::Result {
        let f: LuaFunction = self.serv.get("_message").unwrap();
        f.call::<String>(msg.0).unwrap()
    }
}

pub fn new(file: &str) -> Result<Addr<LuaService>, mlua::Error> {
    let lua = Lua::new();
    let f = std::fs::read_to_string(file).unwrap();
    let load_result = lua.load(&f).eval::<LuaTable>();
    match load_result {
        Ok(serv) => Ok({
            let addr = LuaService::create(|ctx| {         
                let service = LuaService {
                    addr: Some(ctx.address()),
                    _lua: lua,
                    serv,
                };
                service
            });
            addr
        }),
        Err(e) => Err(e),
    }
}
