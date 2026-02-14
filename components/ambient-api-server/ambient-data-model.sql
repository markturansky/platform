-- Ambient Platform Data Model - SQLite Schema
-- Generated from ambient-data-model.md
-- Standardized entity pattern: id, name, repo_url, prompt

-- Drop existing tables if they exist
DROP TABLE IF EXISTS workflow_tasks;
DROP TABLE IF EXISTS workflow_skills;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS skills;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS users;

-- Users table
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    repo_url TEXT,
    prompt TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CHECK(TRIM(name) != '')
);

-- Agents table
CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    repo_url TEXT,
    prompt TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CHECK(TRIM(name) != '')
);

-- Skills table
CREATE TABLE skills (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    repo_url TEXT,
    prompt TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CHECK(TRIM(name) != '')
);

-- Tasks table
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    repo_url TEXT,
    prompt TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CHECK(TRIM(name) != '')
);

-- Workflows table
CREATE TABLE workflows (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    repo_url TEXT,
    prompt TEXT,
    agent_id TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CHECK(TRIM(name) != ''),
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL
);

-- Sessions table
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    repo_url TEXT,
    prompt TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'paused', 'completed', 'archived', 'failed')),
    created_by_user_id TEXT,
    assigned_user_id TEXT,
    workflow_id TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CHECK(TRIM(name) != ''),
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (assigned_user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE SET NULL
);

-- WorkflowSkills junction table (AS agent WITH skill1 skill2)
CREATE TABLE workflow_skills (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    skill_id TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE,
    UNIQUE(workflow_id, skill_id)
);

-- WorkflowTasks junction table (DO task1 task2)
CREATE TABLE workflow_tasks (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    UNIQUE(workflow_id, task_id)
);

-- Indexes for performance
CREATE INDEX idx_users_name ON users(name);
CREATE INDEX idx_agents_name ON agents(name);
CREATE INDEX idx_skills_name ON skills(name);
CREATE INDEX idx_tasks_name ON tasks(name);
CREATE INDEX idx_workflows_name ON workflows(name);
CREATE INDEX idx_workflows_agent_id ON workflows(agent_id);

CREATE INDEX idx_sessions_created_by ON sessions(created_by_user_id);
CREATE INDEX idx_sessions_assigned_to ON sessions(assigned_user_id);
CREATE INDEX idx_sessions_workflow ON sessions(workflow_id);
CREATE INDEX idx_sessions_status ON sessions(status);

CREATE INDEX idx_workflow_skills_workflow ON workflow_skills(workflow_id);
CREATE INDEX idx_workflow_skills_skill ON workflow_skills(skill_id);
CREATE INDEX idx_workflow_skills_position ON workflow_skills(workflow_id, position);

CREATE INDEX idx_workflow_tasks_workflow ON workflow_tasks(workflow_id);
CREATE INDEX idx_workflow_tasks_task ON workflow_tasks(task_id);
CREATE INDEX idx_workflow_tasks_position ON workflow_tasks(workflow_id, position);

-- Triggers for updated_at timestamps
CREATE TRIGGER update_users_updated_at 
AFTER UPDATE ON users FOR EACH ROW
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_agents_updated_at 
AFTER UPDATE ON agents FOR EACH ROW
BEGIN
    UPDATE agents SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_skills_updated_at 
AFTER UPDATE ON skills FOR EACH ROW
BEGIN
    UPDATE skills SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_tasks_updated_at 
AFTER UPDATE ON tasks FOR EACH ROW
BEGIN
    UPDATE tasks SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_workflows_updated_at 
AFTER UPDATE ON workflows FOR EACH ROW
BEGIN
    UPDATE workflows SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_sessions_updated_at 
AFTER UPDATE ON sessions FOR EACH ROW
BEGIN
    UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_workflow_skills_updated_at 
AFTER UPDATE ON workflow_skills FOR EACH ROW
BEGIN
    UPDATE workflow_skills SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_workflow_tasks_updated_at 
AFTER UPDATE ON workflow_tasks FOR EACH ROW
BEGIN
    UPDATE workflow_tasks SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;