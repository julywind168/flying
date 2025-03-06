use mlua::prelude::*;
use std::str::FromStr;

use anyhow::Result;
use mongodb::{
    Client, Collection, Database,
    bson::{self, Bson, Document, oid},
};

struct LuaMongoClient(Client);
struct LuaMongoDatabase(Database);
struct LuaMongoCollection(Collection<Document>);

#[cfg(test)]
mod tests {
    use super::*;
    fn assert_send<T: Send>() {}

    #[test]
    fn expect_lua_send_trait() {
        assert_send::<LuaMongoClient>();
        assert_send::<LuaMongoDatabase>();
        assert_send::<LuaMongoCollection>();
    }
}

impl LuaUserData for LuaMongoClient {
    fn add_methods<M: LuaUserDataMethods<Self>>(methods: &mut M) {
        methods.add_method("database", |_: &Lua, this, dbname: String| {
            Ok(LuaMongoDatabase(this.0.database(&dbname)))
        })
    }
}

impl LuaUserData for LuaMongoDatabase {
    fn add_methods<M: LuaUserDataMethods<Self>>(methods: &mut M) {
        methods.add_method("collection", |_, this, collname: String| {
            Ok(LuaMongoCollection(this.0.collection(&collname)))
        })
    }
}

impl LuaUserData for LuaMongoCollection {
    fn add_methods<M: LuaUserDataMethods<Self>>(methods: &mut M) {
        methods.add_async_method("find_one", |lua, this, filter: LuaTable| async move {
            let f = tbl_to_doc(&lua, filter).map_err(LuaError::external)?;
            let doc = this.0.find_one(f).await.map_err(LuaError::external)?;
            match doc {
                Some(doc) => Ok(Some(to_tbl(&lua, doc).unwrap())),
                None => Ok(None),
            }
        });
        methods.add_async_method("insert_one", |lua, this, t: LuaTable| async move {
            let doc = tbl_to_doc(&lua, t).map_err(LuaError::external)?;
            let res = this.0.insert_one(doc).await.map_err(LuaError::external)?;
            let id = match res.inserted_id.as_object_id() {
                Some(oid) => oid.to_string(),
                None => return Err(LuaError::external("inserted_id is not an ObjectId")),
            };
            Ok(id)
        });
    }
}

pub fn lua_open_flying_mongodb(lua: &Lua, flying: &LuaTable) -> Result<()> {
    let mongodb = lua.create_table()?;
    let connect = lua.create_async_function(async move |_, url: String| {
        let client = mongodb::Client::with_uri_str(url)
            .await
            .map_err(LuaError::external)?;
        Ok(LuaMongoClient(client))
    })?;
    mongodb.set("connect", connect)?;
    flying.set("mongodb", mongodb)?;
    Ok(())
}

fn to_tbl(lua: &Lua, doc: Document) -> Result<mlua::Table> {
    let table: LuaTable = lua.create_table()?;
    for (key, b) in doc {
        table.set(key, to_lua_value(lua, b)?)?;
    }
    Ok(table)
}

fn tbl_to_doc(lua: &Lua, table: mlua::Table) -> Result<Document> {
    let mut doc = Document::new();
    for pair in table.pairs::<mlua::Value, mlua::Value>() {
        let (key, value) = pair?;
        let key = match key {
            mlua::Value::String(s) => s.to_str()?.to_string(),
            mlua::Value::Number(n) => n.to_string(),
            mlua::Value::Integer(i) => i.to_string(),
            _ => anyhow::bail!("tbl_to_doc: invalid key type: {}", key.type_name()),
        };
        let b = to_bson(lua, value, key == "_id")?;
        doc.insert(key, b);
    }
    Ok(doc)
}

fn tbl_to_bson(lua: &Lua, table: mlua::Table) -> Result<Bson> {
    let length = table.len()?;
    if length > 0 {
        let mut array = Vec::new();
        for i in 1..=length {
            match table.raw_get(i) {
                Ok(value) => {
                    array.push(to_bson(lua, value, false)?);
                }
                _ => break,
            }
        }
        if !array.is_empty() {
            return Ok(Bson::Array(array));
        }
    }
    let doc = tbl_to_doc(lua, table)?;
    Ok(Bson::Document(doc))
}

fn to_bson(lua: &Lua, value: mlua::Value, is_object_id: bool) -> Result<Bson> {
    match value {
        mlua::Value::Nil => Ok(Bson::Null),
        mlua::Value::Boolean(b) => Ok(Bson::Boolean(b)),
        mlua::Value::Number(n) => Ok(Bson::Double(n)),
        mlua::Value::Integer(i) => Ok(Bson::Int64(i)),
        mlua::Value::String(s) => {
            if is_object_id {
                Ok(Bson::ObjectId(oid::ObjectId::from_str(
                    &s.to_str()?.to_string(),
                )?))
            } else {
                Ok(Bson::String(s.to_str()?.to_string()))
            }
        }
        mlua::Value::Table(table) => Ok(tbl_to_bson(lua, table)?),
        _ => anyhow::bail!("unsupported type {}", value.type_name()),
    }
}

fn to_lua_value(lua: &Lua, value: bson::Bson) -> Result<mlua::Value> {
    match value {
        bson::Bson::ObjectId(id) => Ok(mlua::Value::String(lua.create_string(&id.to_string())?)),
        bson::Bson::String(s) => Ok(mlua::Value::String(lua.create_string(&s)?)),
        bson::Bson::Int32(i) => Ok(mlua::Value::Integer(i as i64)),
        bson::Bson::Int64(i) => Ok(mlua::Value::Integer(i as i64)),
        bson::Bson::Double(d) => Ok(mlua::Value::Number(d)),
        bson::Bson::Boolean(b) => Ok(mlua::Value::Boolean(b)),
        bson::Bson::Null => Ok(mlua::Value::Nil),
        bson::Bson::Document(doc) => Ok(mlua::Value::Table(to_tbl(lua, doc)?)),
        bson::Bson::Array(arr) => {
            let table = lua.create_table()?;
            for (i, item) in arr.into_iter().enumerate() {
                table.set(i + 1, to_lua_value(lua, item)?)?; // Lua 数组从 1 开始
            }
            Ok(mlua::Value::Table(table))
        }
        _ => anyhow::bail!("unsupported bson type {}", value),
    }
}
