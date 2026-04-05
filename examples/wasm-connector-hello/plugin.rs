//! Squad Aegis WASM connector ABI v1 (minimal invoke demo).
//! Build: rustc --edition 2021 --crate-type cdylib plugin.rs --target wasm32-unknown-unknown -C opt-level=s -o plugin.wasm
//!
//! Responds to ConnectorInvokeRequest JSON containing `"ping"` in the payload with a pong message.

#![no_std]
#![no_main]

#[panic_handler]
fn panic(_: &core::panic::PanicInfo<'_>) -> ! {
    loop {}
}

#[no_mangle]
pub unsafe extern "C" fn aegis_init(_config_off: i32, _config_len: i32) -> i32 {
    0
}

#[no_mangle]
pub unsafe extern "C" fn aegis_start() -> i32 {
    0
}

#[no_mangle]
pub unsafe extern "C" fn aegis_stop() -> i32 {
    0
}

const RESP_OK: &[u8] = br#"{"v":"1","ok":true,"data":{"message":"pong"}}"#;
const RESP_ERR: &[u8] = br#"{"v":"1","ok":false,"error":"expected ping in request"}"#;

const ERR_BUF: i32 = 2;

unsafe fn write_u32_le(ptr: i32, v: u32) {
    let p = ptr as *mut u8;
    p.write(v as u8);
    p.add(1).write((v >> 8) as u8);
    p.add(2).write((v >> 16) as u8);
    p.add(3).write((v >> 24) as u8);
}

unsafe fn write_response(
    out_ptr: i32,
    out_cap: i32,
    out_written_ptr: i32,
    bytes: &[u8],
) -> i32 {
    let n = bytes.len() as i32;
    if n > out_cap {
        return ERR_BUF;
    }
    let out = core::slice::from_raw_parts_mut(out_ptr as *mut u8, out_cap as usize);
    out[..bytes.len()].copy_from_slice(bytes);
    write_u32_le(out_written_ptr, bytes.len() as u32);
    0
}

fn contains_ping(req: &[u8]) -> bool {
    // Loose match for demo: request JSON from host includes "ping" for ping actions.
    for w in req.windows(4) {
        if w == b"ping" {
            return true;
        }
    }
    false
}

#[no_mangle]
pub unsafe extern "C" fn aegis_invoke(
    req_ptr: i32,
    req_len: i32,
    out_ptr: i32,
    out_cap: i32,
    out_written_ptr: i32,
) -> i32 {
    let req = core::slice::from_raw_parts(req_ptr as *const u8, req_len as usize);
    if contains_ping(req) {
        write_response(out_ptr, out_cap, out_written_ptr, RESP_OK)
    } else {
        write_response(out_ptr, out_cap, out_written_ptr, RESP_ERR)
    }
}
