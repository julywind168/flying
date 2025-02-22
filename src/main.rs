use actix::prelude::*;

#[derive(Message)]
#[rtype(result = "usize")]
struct Sum(usize, usize);

struct Calculator;

impl Actor for Calculator {
    type Context = Context<Self>;
}

impl Handler<Sum> for Calculator {
    type Result = usize;
    fn handle(&mut self, msg: Sum, _: &mut Self::Context) -> Self::Result {
        msg.0 + msg.1
    }
}

#[actix::main] // <- starts the system and block until future resolves
async fn main() {
    let addr = Calculator.start();
    let res = addr.send(Sum(10, 5)).await; // <- send message and get future for result

    match res {
        Ok(result) => println!("SUM: {}", result),
        _ => println!("Communication to the actor has failed"),
    }
}
