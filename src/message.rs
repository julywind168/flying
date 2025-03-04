
#[derive(Debug)]
pub enum Message {
    Started,
    Stopping,
    Stopped,
    Request {
        source: String,
        session: u128,
        data: String,
    },
    Response {
        session: u128,
        data: String,
    },
}
