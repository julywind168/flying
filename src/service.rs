use std::{cell::RefCell, collections::HashMap, fs, sync::Arc, time::Duration};

use crate::{message::Message, node::Node};
use mlua::prelude::*;
use tokio::{
    sync::{mpsc, oneshot},
    time::sleep,
};

struct LuaFlying {
    node: Arc<Node>,
    name: String,
    scriptname: String,
    session: RefCell<u128>,
    sessions: RefCell<HashMap<u128, oneshot::Sender<String>>>,
}

impl LuaUserData for LuaFlying {
    fn add_methods<M: mlua::UserDataMethods<Self>>(methods: &mut M) {
        methods.add_method("nodename", |_, this, ()| Ok(this.node.name.clone()));
        methods.add_method("name", |_, this, ()| Ok(this.name.clone()));
        methods.add_method("scriptname", |_, this, ()| Ok(this.scriptname.clone()));
        methods.add_async_method("setenv", async |_, this, (k, v)| {
            Ok(this.node.setenv(k, v).await)
        });
        methods.add_async_method("getenv", async |_, this, k| Ok(this.node.getenv(k).await));
        methods.add_async_method("sleep", async |_, _this, ms| {
            Ok(sleep(Duration::from_millis(ms)).await)
        });
        methods.add_async_method("spawn", async |_, this, (name, scriptname)| {
            Ok(this.node.spawn(name, scriptname).await)
        });
        methods.add_async_method("stop", async |_, this, ()| {
            Ok(this.node.sendto(&this.name, Message::Stopping).await)
        });
        methods.add_async_method("send", async |_, this, (dest, data): (String, String)| {
            Ok(this
                .node
                .sendto(
                    &dest,
                    Message::Request {
                        source: this.name.clone(),
                        session: 0,
                        data,
                    },
                )
                .await)
        });
        methods.add_async_method("call", async |_, this, (dest, data): (String, String)| {
            if let Some(addr) = this.node.query(&dest).await {
                let (tx, rx) = oneshot::channel();
                let session = {
                    *this.session.borrow_mut() += 1;
                    *this.session.borrow()
                };
                this.sessions.borrow_mut().insert(session, tx);

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
        methods.add_method_mut("_wakeup", |_, this, (session, data): (u128, String)| {
            if let Some(tx) = this.sessions.borrow_mut().remove(&session) {
                tx.send(data).into_lua_err()
            } else {
                Err(mlua::Error::RuntimeError(format!(
                    "session {} not found",
                    session
                )))
            }
        });
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
        session: RefCell::new(0),
        sessions: RefCell::new(HashMap::new()),
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
    let fork = lua.create_function(|_, f: LuaFunction| {
        tokio::spawn(async move {
            f.call_async::<()>(()).await.unwrap();
        });
        Ok(())
    });
    flying.set("fork", fork.unwrap()).unwrap();

    let init = flying.get::<LuaFunction>("on_init").unwrap();
    let cb = flying.get::<LuaFunction>("on_event").unwrap();
    (lua, init, cb)
}
