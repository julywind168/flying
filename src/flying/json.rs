use anyhow::Result;
use mlua::prelude::*;

pub fn lua_open_flying_json(lua: &Lua, flying: &LuaTable) -> Result<()> {
    let json = lua.create_table()?;

    let encode = lua.create_function(|_, v: LuaValue| {
        let json = serde_json::to_string(&v).map_err(LuaError::external)?;
        Ok(json)
    })?;

    let decode = lua.create_function(|lua, v: String| {
        let val: serde_json::Value = serde_json::from_str(&v).map_err(LuaError::external)?;
        Ok(lua.to_value(&val).map_err(LuaError::external)?)
    })?;

    json.set("encode", encode)?;
    json.set("decode", decode)?;
    flying.set("json", json)?;
    Ok(())
}
