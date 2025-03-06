use crate::{
    flying::{mongodb::lua_open_flying_mongodb, socket::lua_open_flying_socket},
    message::Message,
    node::Node,
};
use anyhow::Result;
use dashmap::DashMap;
use mlua::prelude::*;
use std::{fs, sync::Arc, time::Duration};
use tokio::{
    sync::{mpsc, oneshot},
    time::sleep,
};

struct LuaFlying {
    node: Arc<Node>,
    name: String,
    scriptname: String,
    sessions: Arc<DashMap<u128, oneshot::Sender<String>>>,
}

#[cfg(test)]
mod tests {
    use super::*;
    fn assert_send<T: Send>() {}

    #[test]
    fn expect_lua_send_trait() {
        assert_send::<LuaFlying>();
    }
}

impl LuaUserData for LuaFlying {
    fn add_methods<M: mlua::UserDataMethods<Self>>(methods: &mut M) {
        methods.add_method("nodename", |_, this, ()| Ok(this.node.name.clone()));
        methods.add_method("name", |_, this, ()| Ok(this.name.clone()));
        methods.add_method("scriptname", |_, this, ()| Ok(this.scriptname.clone()));
        methods.add_method("starttime", |_, this, ()| Ok(this.node.start_time));
        methods.add_method("now", |_, this, ()| Ok(this.node.now()));
        methods.add_method("time", |_, this, ()| Ok(this.node.time()));
        methods.add_method("setenv", |_, this, (k, v): (String, String)| {
            this.node.setenv(k, v);
            Ok(())
        });
        methods.add_method("getenv", |_, this, k: String| Ok(this.node.getenv(&k)));
        methods.add_async_method("sleep", async |_, _this, ms| {
            Ok(sleep(Duration::from_millis(ms)).await)
        });
        methods.add_method("fork", |_, _this, f: LuaFunction| {
            tokio::spawn(async move {
                if let Err(e) = f.call_async::<()>(()).await {
                    log::error!("fork error: {e}");
                }
            });
            Ok(())
        });
        methods.add_async_method(
            "spawn",
            async |_, this, (name, scriptname): (String, String)| {
                this.node
                    .spawn(name, scriptname)
                    .await
                    .map_err(LuaError::external)
            },
        );
        methods.add_async_method("stop", async |_, this, ()| {
            this.node
                .sendto(&this.name, Message::Stopping)
                .await
                .map_err(LuaError::external)
        });
        methods.add_method("send", |_, this, (dest, data): (String, String)| {
            let node = this.node.clone();
            let source = this.name.clone();
            tokio::spawn(async move {
                node.sendto(
                    &dest,
                    Message::Request {
                        source,
                        session: 0,
                        data,
                    },
                )
                .await
            });
            Ok(())
        });
        methods.add_async_method(
            "call",
            async |_, this, (dest, session, data): (String, u128, String)| {
                if let Some(addr) = this.node.query(&dest) {
                    let source = this.name.clone();
                    let (tx, rx) = oneshot::channel();
                    this.sessions.insert(session, tx);
                    tokio::spawn(async move {
                        addr.send(Message::Request {
                            source,
                            session,
                            data,
                        })
                        .await
                    });
                    Ok(rx.await.into_lua_err())
                } else {
                    Err(LuaError::RuntimeError(format!(
                        "service {} not found",
                        dest
                    )))
                }
            },
        );
    }
}

pub type Service = mpsc::Sender<Message>;

pub async fn new(name: String, scriptname: String, node: Arc<Node>) -> Result<Service> {
    let (tx, mut rx) = mpsc::channel(16);
    let _ = tx.send(Message::Started).await;
    let tx2 = tx.clone();
    let sessions = Arc::new(DashMap::new());
    let core = LuaFlying {
        node: node.clone(),
        name: name.clone(),
        scriptname: scriptname.clone(),
        sessions: sessions.clone(),
    };

    let (lua, init, callback) = newlua(&scriptname)?;
    init.call::<()>(core)?;

    tokio::spawn(async move {
        while let Some(msg) = rx.recv().await {
            match msg {
                Message::Request {
                    source,
                    session,
                    data,
                } => {
                    let source = source.clone();
                    let callback = callback.clone();
                    let node = node.clone();
                    tokio::spawn(async move {
                        let data = callback
                            .call_async::<String>(("request", source.clone(), data))
                            .await
                            .unwrap_or("".to_string());
                        if session > 0 {
                            let _ = node
                                .sendto(&source, Message::Response { session, data })
                                .await;
                        }
                    });
                }
                Message::Response { session, data } => {
                    if let Some((_, tx)) = sessions.remove(&session) {
                        if let Err(e) = tx.send(data) {
                            log::error!("handle response: {e}")
                        }
                    } else {
                        log::error!("session not found: {}", session);
                    }
                }
                Message::Started => {
                    let callback = callback.clone();
                    tokio::spawn(async move {
                        if let Err(e) = callback.call_async::<()>("started").await {
                            log::error!("calling started callback: {}", e)
                        }
                    });
                }
                Message::Stopping => {
                    let tx = tx2.clone();
                    let callback = callback.clone();
                    tokio::spawn(async move {
                        if let Err(e) = callback.call_async::<()>("stopping").await {
                            log::error!("calling stopping callback: {}", e);
                        }
                        if let Err(e) = tx.clone().send(Message::Stopped).await {
                            log::error!("sending stopped message: {}", e);
                        }
                    });
                }
                Message::Stopped => {
                    break;
                }
            }
        }
        println!("{} Stopped", name);
        node.remove(&name);
        if let Err(e) = callback.call_async::<()>("stopped").await {
            log::error!("calling {name}.stopped: {e}");
        }
        let _lua = lua;
    });
    Ok(tx)
}

fn newlua(scriptname: &str) -> Result<(Lua, LuaFunction, LuaFunction)> {
    let lua = Lua::new();
    let script = fs::read_to_string(scriptname)?;
    lua.load(&script).exec()?;
    let flying: LuaTable = load_lua_flying(&lua)?;
    lua_open_flying_socket(&lua, &flying)?;
    lua_open_flying_mongodb(&lua, &flying)?;
    let init = flying.get::<LuaFunction>("on_init")?;
    let cb = flying.get::<LuaFunction>("on_event")?;
    Ok((lua, init, cb))
}

fn load_lua_flying(lua: &Lua) -> Result<LuaTable> {
    let flying: LuaTable = lua.load(r#"require "flying""#).eval()?;
    let info = lua.create_function(|_, msg: String| {
        log::info!("{}", msg);
        Ok(())
    })?;
    let warn = lua.create_function(|_, msg: String| {
        log::warn!("{}", msg);
        Ok(())
    })?;
    let error = lua.create_function(|_, msg: String| {
        log::error!("{}", msg);
        Ok(())
    })?;
    let debug = lua.create_function(|_, msg: String| {
        log::debug!("{}", msg);
        Ok(())
    })?;
    let trace = lua.create_function(|_, msg: String| {
        log::trace!("{}", msg);
        Ok(())
    })?;

    flying.set("info", info)?;
    flying.set("warn", warn)?;
    flying.set("error", error)?;
    flying.set("debug", debug)?;
    flying.set("trace", trace)?;

    Ok(flying)
}
