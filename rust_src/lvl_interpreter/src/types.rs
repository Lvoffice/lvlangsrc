use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// ==================== Program types (for JSON deserialization) ====================

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Position {
    #[serde(default)]
    pub file: String,
    #[serde(default = "default_line")]
    pub line: usize,
    #[serde(default = "default_col")]
    pub column: usize,
}

fn default_line() -> usize { 1 }
fn default_col() -> usize { 1 }

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Event {
    #[serde(rename = "Type")]
    pub event_type: String,
    #[serde(rename = "Message", default)]
    pub message: String,
    #[serde(rename = "Actions", default)]
    pub actions: Vec<HashMap<String, serde_json::Value>>,
    #[serde(rename = "Special", default)]
    pub special: String,
    #[serde(rename = "Position", default)]
    pub position: Position,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Role {
    #[serde(rename = "Name")]
    pub name: String,
    #[serde(rename = "Events", default)]
    pub events: Vec<Event>,
    #[serde(rename = "Variables", default)]
    pub variables: HashMap<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Program {
    #[serde(rename = "Roles", default)]
    pub roles: Vec<Role>,
    #[serde(rename = "FilePath", default)]
    pub file_path: String,
    #[serde(rename = "Position", default)]
    pub position: Position,
}
