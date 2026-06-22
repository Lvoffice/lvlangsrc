use super::types::{Event, Program, Role};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::thread;
use std::time::Duration;

/// Per-role runtime state
struct RoleState {
    variables: HashMap<String, f64>,
}

impl RoleState {
    fn new(vars: &HashMap<String, serde_json::Value>) -> Self {
        let mut variables = HashMap::new();
        for (k, v) in vars {
            if let Some(n) = v.as_f64() {
                variables.insert(k.clone(), n);
            }
        }
        RoleState { variables }
    }

    fn get(&self, name: &str) -> Option<f64> {
        self.variables.get(name).copied()
    }

    fn set(&mut self, name: &str, value: f64) {
        self.variables.insert(name.to_string(), value);
    }
}

/// Runtime error collector
struct RuntimeErrorCollector {
    errors: Vec<String>,
    warnings: usize,
    debug: bool,
}

impl RuntimeErrorCollector {
    fn new(debug: bool) -> Self {
        RuntimeErrorCollector { errors: Vec::new(), warnings: 0, debug }
    }

    fn add_error(&mut self, msg: String) {
        self.errors.push(msg);
    }

    fn has_errors(&self) -> bool {
        !self.errors.is_empty()
    }

    fn report(&self) -> String {
        if self.errors.is_empty() && self.warnings == 0 {
            return String::new();
        }
        if self.debug {
            self.errors.join("\n")
        } else {
            format!("运行时发现 {} 个错误", self.errors.len())
        }
    }
}

/// Interpreter
pub struct Interpreter {
    roles: Vec<(String, Arc<RwLock<RoleState>>)>,
    program: Program,
    debug: bool,
    errors: RuntimeErrorCollector,
}

impl Interpreter {
    pub fn new(program: Program, debug: bool) -> Self {
        let mut roles = Vec::new();
        for role in &program.roles {
            let state = Arc::new(RwLock::new(RoleState::new(&role.variables)));
            roles.push((role.name.clone(), state));
        }
        Interpreter {
            roles,
            program,
            debug,
            errors: RuntimeErrorCollector::new(debug),
        }
    }

    pub fn execute(&mut self) -> String {
        let mut handles = Vec::new();

        for role in &self.program.roles {
            let role_name = role.name.clone();
            for event in &role.events {
                if event.event_type == "开始" || event.event_type == "start" {
                    // Clone the Arc for the spawned thread
                    let state_arc = self.roles.iter()
                        .find(|(n, _)| n == &role_name)
                        .map(|(_, s)| s.clone());

                    let role_states: Vec<(String, Arc<RwLock<RoleState>>)> = self.roles.iter()
                        .map(|(n, s)| (n.clone(), s.clone()))
                        .collect();
                    let all_roles: Vec<Role> = self.program.roles.clone();
                    let event = event.clone();
                    let role_name_clone = role_name.clone();
                    let debug = self.debug;

                    let handle = thread::spawn(move || {
                        let mut local_errors = Vec::new();
                        for action in &event.actions {
                            execute_action(&role_name_clone, &state_arc, &action, &all_roles, &role_states, debug, &mut local_errors);
                        }
                        local_errors
                    });
                    handles.push((role_name.clone(), handle));
                }
            }
        }

        let mut all_errors = Vec::new();
        for (role_name, handle) in handles {
            match handle.join() {
                Ok(errors) => {
                    for e in errors {
                        all_errors.push(format!("[{}.开始] {}", role_name, e));
                    }
                }
                Err(_) => {
                    all_errors.push(format!("[{}.开始] 线程崩溃", role_name));
                }
            }
        }

        for e in all_errors {
            self.errors.add_error(e);
        }

        self.errors.report()
    }
}

/// Resolve a value - if it's a variable reference, look it up
fn resolve_value(state: &Option<Arc<RwLock<RoleState>>>, val: &serde_json::Value) -> Option<f64> {
    match val {
        serde_json::Value::Number(n) => Some(n.as_f64().unwrap_or(0.0)),
        serde_json::Value::Object(map) => {
            if map.get("type").and_then(|t| t.as_str()) == Some("variable") {
                let name = map.get("name")?.as_str()?;
                if let Some(s) = state {
                    s.read().unwrap().get(name)
                } else {
                    None
                }
            } else {
                None
            }
        }
        _ => None,
    }
}

/// Resolve value to string
fn resolve_string(val: &serde_json::Value) -> String {
    match val {
        serde_json::Value::String(s) => s.clone(),
        serde_json::Value::Number(n) => n.to_string(),
        _ => String::new(),
    }
}

fn execute_action(
    role_name: &str,
    state: &Option<Arc<RwLock<RoleState>>>,
    action: &HashMap<String, serde_json::Value>,
    all_roles: &[Role],
    role_states: &[(String, Arc<RwLock<RoleState>>)],
    debug: bool,
    errors: &mut Vec<String>,
) {
    let action_type = action.get("type").and_then(|t| t.as_str()).unwrap_or("");

    match action_type {
        "说" => {
            if let Some(content) = action.get("content") {
                let text = resolve_string(content);
                println!("{}", text);
                if debug {
                    println!("[调试] [{}] 说: {}", role_name, text);
                }
            }
        }

        "移动" => {
            if let Some(steps_val) = action.get("steps") {
                if let Some(steps) = resolve_value(state, steps_val) {
                    if let Some(s) = state {
                        let mut st = s.write().unwrap();
                        let current = st.get("x").unwrap_or(0.0);
                        st.set("x", current + steps);
                    }
                    if debug {
                        println!("[调试] [{}] 移动: {}步", role_name, steps);
                    }
                }
            }
        }

        "旋转" => {
            if let Some(angle_val) = action.get("angle") {
                if let Some(angle) = resolve_value(state, angle_val) {
                    if let Some(s) = state {
                        let mut st = s.write().unwrap();
                        let current = st.get("方向").unwrap_or(0.0);
                        st.set("方向", current + angle);
                    }
                    if debug {
                        println!("[调试] [{}] 旋转: {}度", role_name, angle);
                    }
                }
            }
        }

        "等待" => {
            if let Some(sec_val) = action.get("seconds") {
                if let Some(seconds) = resolve_value(state, sec_val) {
                    if debug {
                        println!("[调试] [{}] 等待: {}秒", role_name, seconds);
                    }
                    thread::sleep(Duration::from_secs_f64(seconds));
                }
            }
        }

        "广播" => {
            if let Some(msg_val) = action.get("message") {
                let msg = resolve_string(msg_val);
                if debug {
                    println!("[调试] [{}] 广播: {}", role_name, msg);
                }

                let mut broadcast_handles = Vec::new();
                for role in all_roles {
                    for event in &role.events {
                        if (event.event_type == "收到" || event.event_type == "message") && event.message == msg {
                            let target_name = role.name.clone();
                            let target_event = event.clone();
                            let target_roles: Vec<Role> = all_roles.to_vec();
                            let target_states: Vec<(String, Arc<RwLock<RoleState>>)> = role_states.to_vec();
                            let target_state = target_states.iter()
                                .find(|(n, _)| n == &target_name)
                                .map(|(_, s)| s.clone());

                            let handle = thread::spawn(move || {
                                let mut local_errors = Vec::new();
                                for action in &target_event.actions {
                                    execute_action(&target_name, &target_state, &action, &target_roles, &target_states, debug, &mut local_errors);
                                }
                                local_errors
                            });
                            broadcast_handles.push(handle);
                        }
                    }
                }

                for handle in broadcast_handles {
                    if let Ok(e) = handle.join() {
                        errors.extend(e);
                    }
                }
            }
        }

        "赋值" => {
            if let Some(name_val) = action.get("variable") {
                let name = name_val.as_str().unwrap_or("");
                if let Some(value_val) = action.get("value") {
                    if let Some(value) = resolve_value(state, value_val) {
                        if let Some(s) = state {
                            let mut st = s.write().unwrap();
                            st.set(name, value);
                        }
                        if debug {
                            println!("[调试] [{}] 赋值: {} = {}", role_name, name, value);
                        }
                    }
                }
            }
        }

        _ => {
            if debug {
                errors.push(format!("未知动作类型: {}", action_type));
            }
        }
    }
}

/// Execute from program JSON string. Returns empty string on success, error message on failure.
pub fn execute_from_json(program_json: &str, debug: bool) -> String {
    let program: Program = match serde_json::from_str(program_json) {
        Ok(p) => p,
        Err(e) => return format!("JSON 解析失败: {}", e),
    };

    let mut interp = Interpreter::new(program, debug);
    let report = interp.execute();

    if interp.errors.has_errors() {
        report
    } else {
        String::new()
    }
}
