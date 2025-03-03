use mlua::prelude::*;
use std::{error::Error, str::FromStr};

use mongodb::{
    Client, Collection, Database,
    bson::{self, Bson, Document, oid},
};

struct LuaMongoClient(Client);
struct LuaMongoDatabase(Database);
struct LuaMongoCollection(Collection<Document>);

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
            let f = from_lua_table(&lua, filter).unwrap();
            let doc = this.0.find_one(f).await.unwrap();
            match doc {
                Some(doc) => Ok(Some(to_lua_table(&lua, doc).unwrap())),
                None => Ok(None),
            }
        });
        methods.add_async_method("insert_one", |lua, this, t: LuaTable| async move {
            let doc = from_lua_table(&lua, t).unwrap();
            let res = this.0.insert_one(doc).await.unwrap();
            let id = res.inserted_id.as_object_id().unwrap().to_string();
            Ok(id)
        });
    }
}

pub fn lua_open_flying_mongodb(lua: &Lua, flying: &LuaTable) {
    let mongodb = lua.create_table().unwrap();
    let connect = lua
        .create_async_function(async move |_, url: String| {
            let client = mongodb::Client::with_uri_str(url).await.unwrap();
            Ok(LuaMongoClient(client))
        })
        .unwrap();
    mongodb.set("connect", connect).unwrap();
    flying.set("mongodb", mongodb).unwrap();
}

fn to_lua_table(lua: &Lua, doc: Document) -> Result<mlua::Table, Box<dyn Error>> {
    let table: LuaTable = lua.create_table()?;
    for (key, b) in doc {
        table.set(key, to_lua_value(lua, b)?)?;
    }
    Ok(table)
}

fn from_lua_table(lua: &Lua, table: mlua::Table) -> Result<Document, Box<dyn Error>> {
    let mut doc = Document::new();

    // 遍历表格的键值对
    for pair in table.pairs::<mlua::Value, mlua::Value>() {
        let (key, value) = pair?;

        // 将键转换为字符串（MongoDB 的键必须是字符串）
        let key = match key {
            mlua::Value::String(s) => s.to_str()?.to_string(),
            mlua::Value::Number(n) => n.to_string(),
            mlua::Value::Integer(i) => i.to_string(),
            _ => return Err(format!("Invalid key type: {}", key.type_name()).into()),
        };

        // 将值转换为 BSON
        let bval = from_lua_value(lua, value, key == "_id")?;
        doc.insert(key, bval);
    }

    // 处理数组（Lua 中以连续整数键 1 开始的表视为数组）
    if table
        .clone()
        .pairs::<mlua::Value, mlua::Value>()
        .next()
        .is_none()
    {
        let mut array = Vec::new();
        for i in 1.. {
            match table.raw_get(i) {
                Ok(mlua::Value::Nil) => break,
                Ok(value) => {
                    let bson_value = from_lua_value(lua, value, false)?;
                    array.push(bson_value);
                }
                Err(_) => break,
            }
        }
        if !array.is_empty() {
            return Ok(bson::doc! { "array": Bson::Array(array) });
        }
    }

    Ok(doc)
}

fn from_lua_value(
    lua: &Lua,
    value: mlua::Value,
    is_object_id: bool,
) -> Result<Bson, Box<dyn Error>> {
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
        mlua::Value::Table(table) => Ok(Bson::Document(from_lua_table(lua, table)?)),
        _ => Err(format!("Unsupported Lua type: {}", value.type_name()).into()),
    }
}

fn to_lua_value(lua: &Lua, value: bson::Bson) -> Result<mlua::Value, Box<dyn Error>> {
    match value {
        bson::Bson::ObjectId(id) => Ok(mlua::Value::String(lua.create_string(&id.to_string())?)),
        bson::Bson::String(s) => Ok(mlua::Value::String(lua.create_string(&s)?)),
        bson::Bson::Int32(i) => Ok(mlua::Value::Integer(i as i64)),
        bson::Bson::Int64(i) => Ok(mlua::Value::Integer(i as i64)),
        bson::Bson::Double(d) => Ok(mlua::Value::Number(d)),
        bson::Bson::Boolean(b) => Ok(mlua::Value::Boolean(b)),
        bson::Bson::Null => Ok(mlua::Value::Nil),
        bson::Bson::Document(doc) => Ok(mlua::Value::Table(to_lua_table(lua, doc)?)),
        bson::Bson::Array(arr) => {
            let table = lua.create_table()?;
            for (i, item) in arr.into_iter().enumerate() {
                table.set(i + 1, to_lua_value(lua, item)?)?; // Lua 数组从 1 开始
            }
            Ok(mlua::Value::Table(table))
        }
        _ => Err(format!("Unsupported BSON type: {:?}", value).into()),
    }
}
