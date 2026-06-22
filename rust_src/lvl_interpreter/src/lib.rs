mod types;
mod interpreter;

use std::ffi::{CStr, CString};
use std::os::raw::c_char;

/// Execute program from JSON. Returns {"ok":true,"output":"执行完成"} on success.
#[no_mangle]
pub extern "C" fn Execute(program_json: *const c_char, debug: bool) -> *mut c_char {
    let result = do_execute(program_json, debug);
    match CString::new(result) {
        Ok(cstr) => cstr.into_raw(),
        Err(_) => CString::new(r#"{"ok":false,"errors":"返回值编码失败"}"#).unwrap().into_raw(),
    }
}

fn do_execute(program_json: *const c_char, debug: bool) -> String {
    let json_str = unsafe {
        if program_json.is_null() { return r#"{"ok":false,"errors":"程序JSON为空"}"#.to_string(); }
        CStr::from_ptr(program_json).to_string_lossy().into_owned()
    };

    let report = interpreter::execute_from_json(&json_str, debug);

    if report.is_empty() {
        r#"{"ok":true,"output":"执行完成"}"#.to_string()
    } else {
        let escaped = report.replace('\\', "\\\\").replace('"', "\\\"").replace('\n', "\\n");
        format!(r#"{{"ok":false,"errors":"{}"}}"#, escaped)
    }
}

/// Free a string returned by Execute
#[no_mangle]
pub extern "C" fn FreeString(ptr: *mut c_char) {
    if !ptr.is_null() {
        unsafe { let _ = CString::from_raw(ptr); }
    }
}
