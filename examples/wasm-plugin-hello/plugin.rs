//! Squad Aegis WASM plugin ABI v1 (minimal demo).
//! Build: rustc --edition 2021 --crate-type cdylib plugin.rs --target wasm32-unknown-unknown -C opt-level=s -o plugin.wasm

#![no_std]
#![no_main]

#[panic_handler]
fn panic(_: &core::panic::PanicInfo<'_>) -> ! {
    loop {}
}

#[link(wasm_import_module = "aegis_host_v1")]
extern "C" {
    fn log(level: i32, ptr: i32, len: i32);
}

#[no_mangle]
pub unsafe extern "C" fn aegis_init(_config_off: i32, _config_len: i32) -> i32 {
    0
}

#[inline(never)]
#[no_mangle]
pub unsafe extern "C" fn aegis_start() -> i32 {
    0
}

#[inline(never)]
#[no_mangle]
pub unsafe extern "C" fn aegis_stop() -> i32 {
    core::hint::black_box(0)
}

#[no_mangle]
pub unsafe extern "C" fn aegis_on_event(
    _type_off: i32,
    _type_len: i32,
    payload_off: i32,
    payload_len: i32,
) -> i32 {
    log(0, payload_off, payload_len);
    0
}
