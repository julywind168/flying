use std::io;
use std::net::SocketAddr;

use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpListener, TcpStream};

use mlua::{BString, prelude::*};

pub struct LuaTcpListener(TcpListener);
pub struct LuaTcpStream(TcpStream);

impl LuaUserData for LuaTcpListener {
    fn add_methods<M: LuaUserDataMethods<Self>>(methods: &mut M) {
        methods.add_async_method_mut("accept", |_, this, ()| async move {
            let (stream, _) = this.0.accept().await?;
            Ok(LuaTcpStream(stream))
        });
    }
}

impl LuaUserData for LuaTcpStream {
    fn add_methods<M: LuaUserDataMethods<Self>>(methods: &mut M) {
        methods.add_method("peer_addr", |_, this, ()| {
            Ok(this.0.peer_addr()?.to_string())
        });

        methods.add_async_method_mut("read", |lua, mut this, size| async move {
            let mut buf = vec![0; size];
            let n = this.0.read(&mut buf).await?;
            buf.truncate(n);
            lua.create_string(&buf)
        });

        methods.add_async_method_mut("write", |_, mut this, data: BString| async move {
            let n = this.0.write(&data).await?;
            Ok(n)
        });

        methods.add_async_method_mut("close", |_, mut this, ()| async move {
            this.0.shutdown().await?;
            Ok(())
        });
    }
}

pub fn is_transient_error(e: &io::Error) -> bool {
    e.kind() == io::ErrorKind::ConnectionRefused
        || e.kind() == io::ErrorKind::ConnectionAborted
        || e.kind() == io::ErrorKind::ConnectionReset
}

pub fn lua_open_flying_socket(lua: &Lua, flying: &LuaTable) {
    let socket = lua.create_table().unwrap();
    let listen = lua
        .create_async_function(|_, addr: String| async move {
            let addr: SocketAddr = addr.parse::<SocketAddr>()?;
            let listener = TcpListener::bind(addr).await?;
            Ok(LuaTcpListener(listener))
        })
        .unwrap();
    socket.set("listen", listen).unwrap();

    flying.set("socket", socket).unwrap();
}
