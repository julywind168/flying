use std::{collections::HashMap, fs, sync::Arc, time::Duration};

use crate::{message::Message, node::Node};
use mlua::prelude::*;
use tokio::{
    sync::{Mutex, RwLock, mpsc, oneshot},
    time::sleep,
};

struct LuaFlying {
    node: Arc<Node>,
    name: String,
    scriptname: String,
    session: Mutex<u128>,
    sessions: RwLock<HashMap<u128, oneshot::Sender<String>>>,
}

impl LuaUserData for LuaFlying {
    fn add_methods<M: mlua::UserDataMethods<Self>>(methods: &mut M) {
        methods.add_method("nodename", |_, this, ()| Ok(this.node.name.clone()));
        methods.add_method("name", |_, this, ()| Ok(this.name.clone()));
        methods.add_method("scriptname", |_, this, ()| Ok(this.scriptname.clone()));
        methods.add_method("starttime", |_, this, ()| Ok(this.node.start_time));
        methods.add_method("now", |_, this, ()| Ok(this.node.now()));
        methods.add_method("time", |_, this, ()| Ok(this.node.time()));
        methods.add_async_method("setenv", async |_, this, (k, v)| {
            Ok(this.node.setenv(k, v).await)
        });
        methods.add_async_method("getenv", async |_, this, k| Ok(this.node.getenv(k).await));
        methods.add_async_method("sleep", async |_, _this, ms| {
            Ok(sleep(Duration::from_millis(ms)).await)
        });
        methods.add_async_method("fork", async |_, _this, f: LuaFunction| {
            tokio::spawn(async move {
                f.call_async::<()>(()).await.unwrap();
            });
            Ok(())
        });
        methods.add_async_method("spawn", async |_, this, (name, scriptname)| {
            Ok(this.node.spawn(name, scriptname).await)
        });
        methods.add_async_method("stop", async |_, this, ()| {
            Ok(this.node.sendto(&this.name, Message::Stopping).await)
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
        methods.add_async_method("call", async |_, this, (dest, data): (String, String)| {
            if let Some(addr) = this.node.query(&dest).await {
                let (tx, rx) = oneshot::channel();
                let session = {
                    let mut session = this.session.lock().await;
                    *session += 1;
                    *session
                };
                {
                    let mut sessions = this.sessions.write().await;
                    sessions.insert(session, tx);
                };
                tokio::spawn(async move {
                    addr.send(Message::Request {
                        source: this.name.clone(),
                        session,
                        data,
                    })
                    .await
                });
                Ok(rx.await.into_lua_err())
            } else {
                Err(mlua::Error::RuntimeError(format!(
                    "service {} not found",
                    dest
                )))
            }
        });
        methods.add_async_method(
            "_wakeup",
            |_, this, (session, data): (u128, String)| async move {
                if let Some(tx) = this.sessions.write().await.remove(&session) {
                    tx.send(data).into_lua_err()
                } else {
                    Err(mlua::Error::RuntimeError(format!(
                        "session {} not found",
                        session
                    )))
                }
            },
        );
        methods.add_async_method(
            "_respond",
            |_, this, (dest, session, data): (String, u128, String)| async move {
                Ok(this
                    .node
                    .sendto(
                        &dest,
                        Message::Response {
                            source: this.name.clone(),
                            session,
                            data,
                        },
                    )
                    .await)
            },
        );
    }
}

pub type Service = mpsc::Sender<Message>;

pub async fn new(name: String, scriptname: String, node: Arc<Node>) -> Service {
    let (tx, mut rx) = mpsc::channel(16);
    let _ = tx.send(Message::Started).await;
    let tx2 = tx.clone();
    let core = LuaFlying {
        node: node.clone(),
        name: name.clone(),
        scriptname: scriptname.clone(),
        session: Mutex::new(0),
        sessions: RwLock::new(HashMap::new()),
    };

    let (lua, init, callback) = newlua(&scriptname);
    init.call::<()>(core).unwrap();

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
                    tokio::spawn(async move {
                        callback
                            .call_async::<String>(("request", source.clone(), session, data))
                            .await
                            .unwrap_or("".to_string())
                    });
                }
                Message::Response {
                    source,
                    session,
                    data,
                } => {
                    let callback = callback.clone();
                    tokio::spawn(async move {
                        callback
                            .call_async::<()>(("response", source, session, data))
                            .await
                            .unwrap();
                    });
                }
                Message::Started => {
                    let callback = callback.clone();
                    tokio::spawn(async move {
                        callback.call_async::<()>("started").await.unwrap();
                    });
                }
                Message::Stopping => {
                    let tx = tx2.clone();
                    let callback = callback.clone();
                    tokio::spawn(async move {
                        callback.call_async::<()>("stopping").await.unwrap();
                        tx.clone().send(Message::Stopped).await.unwrap();
                    });
                }
                Message::Stopped => {
                    break;
                }
            }
        }
        println!("{} Stopped", name);
        node.remove(&name).await;
        callback.call_async::<()>("stopped").await.unwrap();
        let _lua = lua;
    });
    tx
}

fn newlua(scriptname: &str) -> (Lua, LuaFunction, LuaFunction) {
    let lua = Lua::new();
    let script = fs::read_to_string(scriptname).unwrap();
    lua.load(&script).exec().unwrap();
    let flying: LuaTable = lua.load(r#"require "flying""#).eval().unwrap();
    let init = flying.get::<LuaFunction>("on_init").unwrap();
    let cb = flying.get::<LuaFunction>("on_event").unwrap();
    (lua, init, cb)
}
