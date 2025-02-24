mod service;

#[actix::main]
async fn main() {
    let _ = service::new("service/main.lua".to_string(), 0);
}
