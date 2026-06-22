mod types;
mod lexer;
mod parser;

use std::collections::HashMap;
use std::ffi::{CStr, CString};
use std::os::raw::c_char;

use types::{ErrorCollector, Program};

/// Parse source code and return JSON string.
/// Returns {"ok":true,"program":{...}} on success, {"ok":false,"errors":"...","error_count":N} on failure.
#[no_mangle]
pub extern "C" fn Parse(source: *const c_char, keywords_json: *const c_char, debug: bool) -> *mut c_char {
    let result = do_parse(source, keywords_json, debug);
    match CString::new(result) {
        Ok(cstr) => cstr.into_raw(),
        Err(_) => CString::new("{\"ok\":false,\"errors\":\"返回值编码失败\"}").unwrap().into_raw(),
    }
}

fn do_parse(source: *const c_char, keywords_json: *const c_char, debug: bool) -> String {
    let source_str = unsafe {
        if source.is_null() { return r#"{"ok":false,"errors":"源码为空"}"#.to_string(); }
        CStr::from_ptr(source).to_string_lossy().into_owned()
    };

    let keywords_str = unsafe {
        if keywords_json.is_null() { return r#"{"ok":false,"errors":"关键词JSON为空"}"#.to_string(); }
        CStr::from_ptr(keywords_json).to_string_lossy().into_owned()
    };

    // Parse keywords JSON: {"keywords": {"角色": ["角色", "sprite"], ...}}
    let grammar: HashMap<String, Vec<String>> = match serde_json::from_str::<serde_json::Value>(&keywords_str) {
        Ok(v) => {
            if let Some(kw) = v.get("keywords").and_then(|k| k.as_object()) {
                kw.iter()
                    .map(|(k, v)| {
                        let words = v.as_array()
                            .map(|arr| arr.iter().filter_map(|s| s.as_str().map(String::from)).collect())
                            .unwrap_or_default();
                        (k.clone(), words)
                    })
                    .collect()
            } else {
                return r#"{"ok":false,"errors":"keywords JSON 格式错误: 缺少 keywords 字段"}"#.to_string();
            }
        }
        Err(e) => {
            return format!(r#"{{"ok":false,"errors":"keywords JSON 解析失败: {}"}}"#, e);
        }
    };

    let mut parser = parser::Parser::new(&source_str, "<input>", &grammar, debug);
    match parser.parse() {
        Ok(program) => {
            let program_json = serde_json::to_string(&program).unwrap_or_else(|e| {
                format!(r#"{{"error":"JSON 序列化失败: {}"}}"#, e)
            });
            format!(r#"{{"ok":true,"program":{}}}"#, program_json)
        }
        Err(errors) => {
            let error_count = errors.errors.iter().filter(|e| e.level >= types::ErrorLevel::Error).count();
            let report = errors.report();
            let errors_json = serde_json::to_string(&errors.errors).unwrap_or_else(|_| r#"[]"#.to_string());
            format!(r#"{{"ok":false,"errors":{},"error_count":{},"report":"{}"}}"#, 
                errors_json, error_count, report.replace('"', "\\\"").replace('\n', "\\n"))
        }
    }
}

/// Free a string returned by Parse or Execute
#[no_mangle]
pub extern "C" fn FreeString(ptr: *mut c_char) {
    if !ptr.is_null() {
        unsafe { let _ = CString::from_raw(ptr); }
    }
}
