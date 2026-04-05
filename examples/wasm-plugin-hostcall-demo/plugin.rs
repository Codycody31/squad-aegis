//! Calls `aegis_host_v1.host_call` with category `server` / method `GetServerID` on `aegis_start`.
//! Build: rustc --edition 2021 --crate-type cdylib plugin.rs --target wasm32-unknown-unknown -C opt-level=s -o plugin.wasm
//!
//! See [docs/wasm-guest-abi.md](../../docs/wasm-guest-abi.md) for the full ABI.

#![no_std]
#![no_main]

#[panic_handler]
fn panic(_: &core::panic::PanicInfo<'_>) -> ! {
    loop {}
}

#[link(wasm_import_module = "aegis_host_v1")]
extern "C" {
    fn host_call(
        cat_ptr: i32,
        cat_len: i32,
        method_ptr: i32,
        method_len: i32,
        req_ptr: i32,
        req_len: i32,
        out_ptr: i32,
        out_cap: i32,
        out_written_ptr: i32,
    ) -> i32;
}

static CAT: &[u8] = b"server";
static METHOD: &[u8] = b"GetServerID";

#[no_mangle]
pub static mut HOSTCALL_OUT: [u8; 512] = [0u8; 512];
#[no_mangle]
pub static mut HOSTCALL_WRITTEN: u32 = 0;

#[no_mangle]
pub unsafe extern "C" fn aegis_init(_config_off: i32, _config_len: i32) -> i32 {
    0
}

#[no_mangle]
pub unsafe extern "C" fn aegis_start() -> i32 {
    let code = host_call(
        CAT.as_ptr() as i32,
        CAT.len() as i32,
        METHOD.as_ptr() as i32,
        METHOD.len() as i32,
        0,
        0,
        HOSTCALL_OUT.as_mut_ptr() as i32,
        HOSTCALL_OUT.len() as i32,
        core::ptr::addr_of_mut!(HOSTCALL_WRITTEN) as i32,
    );
    if code != 0 {
        return code as i32;
    }
    0
}

#[no_mangle]
pub unsafe extern "C" fn aegis_stop() -> i32 {
    0
}

#[no_mangle]
pub unsafe extern "C" fn aegis_on_event(
    _type_off: i32,
    _type_len: i32,
    _payload_off: i32,
    _payload_len: i32,
) -> i32 {
    0
}
