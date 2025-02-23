mod service;

#[actix::main]
async fn main() {
    let login = service::new("service/login.lua", 0);
    let res = login.send(service::Call("[\"hello\", \"world\"]".to_string())).await;
    match res {
        Ok(result) => println!("result: {}", result),
        _ => println!("Communication to the actor has failed"),
    }
}
