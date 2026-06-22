use std::ffi::{CStr, CString};
use std::os::raw::c_char;
use std::path::{Path, PathBuf};

// DLL function types
type ParseFn = unsafe extern "C" fn(*const c_char, *const c_char, bool) -> *mut c_char;
type ExecuteFn = unsafe extern "C" fn(*const c_char, bool) -> *mut c_char;
type FreeStringFn = unsafe extern "C" fn(*mut c_char);

struct DllContext {
    parse: ParseFn,
    execute: ExecuteFn,
    free_string: FreeStringFn,
}

/// Search for a file from exe directory upward: ./ ../ ../libs/
fn search_file(name: &str, exe_dir: &Path) -> Option<PathBuf> {
    let candidates = [
        exe_dir.join(name),
        exe_dir.join("..").join(name),
        exe_dir.join("..").join("libs").join(name),
    ];
    for p in &candidates {
        if p.exists() {
            return Some(p.canonicalize().unwrap_or(p.clone()));
        }
    }
    None
}

#[cfg(windows)]
fn load_dlls(exe_dir: &Path) -> Result<DllContext, String> {
    use windows::Win32::System::LibraryLoader::{GetProcAddress, LoadLibraryA};
    use windows::core::PCSTR;
    use std::mem::transmute;

    let parser_path = search_file("lvl_parser.dll", exe_dir)
        .ok_or_else(|| "找不到 lvl_parser.dll".to_string())?;
    let interp_path = search_file("lvl_interpreter.dll", exe_dir)
        .ok_or_else(|| "找不到 lvl_interpreter.dll".to_string())?;

    unsafe {
        let parser_cstr = CString::new(parser_path.to_str().unwrap()).unwrap();
        let interp_cstr = CString::new(interp_path.to_str().unwrap()).unwrap();

        let parser_dll = LoadLibraryA(PCSTR(parser_cstr.as_ptr() as *const u8))
            .map_err(|e| format!("加载 lvl_parser.dll 失败: {} ({})", e, parser_path.display()))?;
        let interp_dll = LoadLibraryA(PCSTR(interp_cstr.as_ptr() as *const u8))
            .map_err(|e| format!("加载 lvl_interpreter.dll 失败: {} ({})", e, interp_path.display()))?;

        let parse: ParseFn = GetProcAddress(parser_dll, PCSTR(b"Parse\0".as_ptr()))
            .map(|p| transmute(p))
            .ok_or("获取 Parse 函数失败")?;
        let execute: ExecuteFn = GetProcAddress(interp_dll, PCSTR(b"Execute\0".as_ptr()))
            .map(|p| transmute(p))
            .ok_or("获取 Execute 函数失败")?;
        // FreeString from parser dll (both export it, pick one)
        let free_string: FreeStringFn = GetProcAddress(parser_dll, PCSTR(b"FreeString\0".as_ptr()))
            .map(|p| transmute(p))
            .ok_or("获取 FreeString 函数失败")?;

        Ok(DllContext { parse, execute, free_string })
    }
}

fn call_and_free(ctx: &DllContext, ptr: *mut c_char) -> String {
    let result = unsafe {
        let cstr = CStr::from_ptr(ptr);
        cstr.to_string_lossy().into_owned()
    };
    unsafe { (ctx.free_string)(ptr); }
    result
}

fn main() {
    // Set console to UTF-8
    #[cfg(windows)]
    unsafe {
        use windows::Win32::System::Console::SetConsoleOutputCP;
        let _ = SetConsoleOutputCP(65001);
    }

    let args: Vec<String> = std::env::args().collect();
    if args.len() < 2 {
        println!("用法: lvl [-d] <file.lvls>");
        return;
    }

    let debug = args.iter().any(|a| a == "-d");
    let filename = args.iter().skip(1).find(|a| !a.starts_with('-')).cloned();
    let filename = match filename {
        Some(f) => f,
        None => { println!("错误: 请指定文件名"); return; }
    };

    // Find exe directory
    let exe_dir = std::env::current_exe()
        .ok()
        .and_then(|p| p.parent().map(|d| d.to_path_buf()))
        .unwrap_or_else(|| PathBuf::from("."));

    // Find grammar.json
    let grammar_path = search_file("grammar.json", &exe_dir)
        .unwrap_or_else(|| {
            println!("错误: 找不到 grammar.json");
            std::process::exit(1);
        });

    // Find and load DLLs
    let ctx = match load_dlls(&exe_dir) {
        Ok(c) => c,
        Err(e) => { println!("错误: {}", e); return; }
    };

    // Read source file
    let source = match std::fs::read_to_string(&filename) {
        Ok(s) => s,
        Err(e) => { println!("错误: 无法读取文件 '{}': {}", filename, e); return; }
    };

    // Read grammar.json
    let grammar_json = match std::fs::read_to_string(&grammar_path) {
        Ok(s) => s,
        Err(e) => { println!("错误: 无法读取 grammar.json: {}", e); return; }
    };

    // Cache handling
    let abs_path = std::fs::canonicalize(&filename).unwrap_or_else(|_| PathBuf::from(&filename));
    let cache_key = abs_path.to_str().unwrap().replace("\\", "_").replace("/", "_").replace(":", "_");
    let cache_dir = exe_dir.join("..").join("cache");
    let cache_file = cache_dir.join(format!("{}.json", cache_key));

    // Check if cache is valid
    let use_cache = if cache_file.exists() {
        let src_mtime = std::fs::metadata(&filename).and_then(|m| m.modified()).ok();
        let cache_mtime = std::fs::metadata(&cache_file).and_then(|m| m.modified()).ok();
        match (src_mtime, cache_mtime) {
            (Some(s), Some(c)) if c >= s => true,
            _ => false,
        }
    } else {
        false
    };

    let program_json = if use_cache {
        if debug { println!("[缓存] 使用缓存: {:?}", cache_file); }
        match std::fs::read_to_string(&cache_file) {
            Ok(s) => s,
            Err(_) => String::new(),
        }
    } else {
        // Call Parse DLL
        let source_c = CString::new(source.as_str()).unwrap();
        let grammar_c = CString::new(grammar_json.as_str()).unwrap();
        let result_ptr = unsafe { (ctx.parse)(source_c.as_ptr(), grammar_c.as_ptr(), debug) };
        let parse_result = call_and_free(&ctx, result_ptr);

        // Parse the result JSON
        if let Ok(v) = serde_json::from_str::<serde_json::Value>(&parse_result) {
            if v.get("ok").and_then(|o| o.as_bool()) == Some(true) {
                if let Some(program) = v.get("program") {
                    let program_str = serde_json::to_string_pretty(program).unwrap_or_default();
                    // Save cache
                    if let Some(parent) = cache_file.parent() {
                        let _ = std::fs::create_dir_all(parent);
                    }
                    let _ = std::fs::write(&cache_file, &program_str);
                    if debug { println!("[缓存] 已保存: {:?}", cache_file); }
                    program_str
                } else {
                    println!("错误: 解析结果缺少 program 字段");
                    return;
                }
            } else {
                let error_count = v.get("error_count").and_then(|n| n.as_u64()).unwrap_or(0);
                let report = v.get("report").and_then(|r| r.as_str()).unwrap_or("");
                println!("错误: {}", report);
                if !report.is_empty() && report.contains("错误") {
                    // report already has the errors
                } else if error_count > 0 {
                    println!("发现 {} 个错误", error_count);
                }
                return;
            }
        } else {
            println!("解析失败: {}", parse_result);
            return;
        }
    };

    if debug {
        println!("程序 JSON:\n{}", program_json);
    }

    // Call Execute DLL
    let program_c = CString::new(program_json.as_str()).unwrap();
    let result_ptr = unsafe { (ctx.execute)(program_c.as_ptr(), debug) };
    let exec_result = call_and_free(&ctx, result_ptr);

    // Parse execute result, only show errors
    if let Ok(v) = serde_json::from_str::<serde_json::Value>(&exec_result) {
        if v.get("ok").and_then(|o| o.as_bool()) == Some(true) {
            // Success — no output needed (the program itself already printed via "说")
        } else {
            let errors = v.get("errors").and_then(|e| e.as_str()).unwrap_or("执行失败");
            println!("执行错误: {}", errors);
        }
    } else if !exec_result.is_empty() {
        println!("{}", exec_result);
    }
}
