use std::ffi::{CStr, CString};
use std::os::raw::c_char;
use std::path::PathBuf;

type ParseFn = unsafe extern "C" fn(*const c_char, *const c_char, bool) -> *mut c_char;
type FreeStringFn = unsafe extern "C" fn(*mut c_char);

fn search_file(name: &str, exe_dir: &PathBuf) -> Option<PathBuf> {
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

fn main() {
    // Set console to UTF-8
    #[cfg(windows)]
    unsafe {
        use windows::Win32::System::Console::SetConsoleOutputCP;
        let _ = SetConsoleOutputCP(65001);
    }

    let args: Vec<String> = std::env::args().collect();
    if args.len() < 2 {
        println!("用法: lvlp [-d] <file.lvls>");
        println!("  -d    显示详细调试信息");
        return;
    }

    let debug = args.iter().any(|a| a == "-d");
    let filename = args.iter().skip(1).find(|a| !a.starts_with('-')).cloned();
    let filename = match filename {
        Some(f) => f,
        None => { println!("错误: 请指定文件名"); return; }
    };

    let exe_dir = std::env::current_exe()
        .ok()
        .and_then(|p| p.parent().map(|d| d.to_path_buf()))
        .unwrap_or_else(|| PathBuf::from("."));

    let grammar_path = search_file("grammar.json", &exe_dir)
        .unwrap_or_else(|| {
            println!("错误: 找不到 grammar.json");
            std::process::exit(1);
        });

    let parser_path = search_file("lvl_parser.dll", &exe_dir)
        .unwrap_or_else(|| {
            println!("错误: 找不到 lvl_parser.dll");
            std::process::exit(1);
        });

    // Load DLL
    #[cfg(windows)]
    unsafe {
        use windows::Win32::System::LibraryLoader::{GetProcAddress, LoadLibraryA};
        use windows::core::PCSTR;
        use std::mem::transmute;

        let dll_cstr = CString::new(parser_path.to_str().unwrap()).unwrap();
        let dll = match LoadLibraryA(PCSTR(dll_cstr.as_ptr() as *const u8)) {
            Ok(d) => d,
            Err(e) => { println!("错误: 加载 lvl_parser.dll 失败: {}", e); return; }
        };

        let parse_fn: ParseFn = match GetProcAddress(dll, PCSTR(b"Parse\0".as_ptr())) {
            Some(p) => transmute(p),
            None => { println!("错误: 获取 Parse 函数失败"); return; }
        };
        let free_fn: FreeStringFn = match GetProcAddress(dll, PCSTR(b"FreeString\0".as_ptr())) {
            Some(p) => transmute(p),
            None => { println!("错误: 获取 FreeString 函数失败"); return; }
        };

        // Read source
        let source = match std::fs::read_to_string(&filename) {
            Ok(s) => s,
            Err(e) => { println!("错误: 无法读取文件 '{}': {}", filename, e); return; }
        };

        let grammar_json = match std::fs::read_to_string(&grammar_path) {
            Ok(s) => s,
            Err(e) => { println!("错误: 无法读取 grammar.json: {}", e); return; }
        };

        let source_c = CString::new(source.as_str()).unwrap();
        let grammar_c = CString::new(grammar_json.as_str()).unwrap();
        let result_ptr = parse_fn(source_c.as_ptr(), grammar_c.as_ptr(), debug);

        let result = {
            let cstr = CStr::from_ptr(result_ptr);
            cstr.to_string_lossy().into_owned()
        };
        free_fn(result_ptr);

        println!("{}", result);
    }

    #[cfg(not(windows))]
    println!("非 Windows 平台暂不支持");
}
