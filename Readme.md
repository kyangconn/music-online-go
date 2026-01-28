# Music Online Backend (Go) / 在线音乐后台服务 (Go)

[English](#english) | [中文](#chinese)

<a name="english"></a>
## English

### Introduction
This project is a rewrite of a small Java web service using **Go (Golang)**. It serves as the backend for an online music platform, providing RESTful APIs for user management and music content management.

### Tech Stack
- **Language**: Go 1.24+
- **Web Framework**: [Gin](https://github.com/gin-gonic/gin)
- **Database ORM**: [GORM](https://gorm.io/)
- **Database**: PostgreSQL
- **Authentication**: JWT (JSON Web Tokens)
- **Configuration**: Viper (YAML)

### Features

#### 👤 User Module
- **Registration & Login**: Secure user sign-up and login with JWT token issuance.
- **Profile Management**: View and update user profile (Avatar, Bio, etc.).
- **Security**: Password encryption and change password functionality.
- **Role-based Access**: Basic support for user roles (e.g., Admin).

#### 🎵 Music Module
- **Music Management**: Create (Upload), Update, Delete, and Get details of music tracks.
- **Search**: Search music by title or artist.
- **Interactions**: Like (Collect) and Unlike music.
- **User Collections**: View music uploaded by specific users and music liked by specific users.
- **Data Model**: Supports both Singles and Albums (via `type` field).

### Project Structure
```
music-online-go/
├── cmd/
│   └── server/          # Application entry point
├── internal/
│   ├── config/          # Configuration loading
│   ├── domain/          # Data models and DTOs
│   ├── handler/         # HTTP request handlers (Controllers)
│   ├── middleware/      # Gin middlewares (Auth, Logger, CORS)
│   ├── pkg/             # Shared packages (DB, JWT, Utils)
│   ├── repository/      # Data access layer
│   └── service/         # Business logic layer
└── config-example.yaml  # Configuration template
```

### Getting Started

1. **Prerequisites**
   - Go 1.24 or higher
   - PostgreSQL installed and running

2. **Configuration**
   Copy `config-example.yaml` to `config.yaml` and update your database credentials:
   ```bash
   cp config-example.yaml config.yaml
   ```

3. **Run the Application**
   ```bash
   go run cmd/server/main.go
   ```
   The server will start on port `8080` (default).

---

<a name="chinese"></a>
## 中文 (Chinese)

### 简介
本项目是一个小型 Java Web 服务的 **Go (Golang)** 重构版本。它是一个在线音乐平台的后端服务，提供用户管理和音乐内容管理的 RESTful API 接口。

### 技术栈
- **编程语言**: Go 1.24+
- **Web 框架**: [Gin](https://github.com/gin-gonic/gin)
- **ORM 框架**: [GORM](https://gorm.io/)
- **数据库**: PostgreSQL
- **认证方式**: JWT (JSON Web Tokens)
- **配置管理**: Viper (YAML)

### 功能特性

#### 👤 用户模块 (User)
- **注册与登录**: 安全的用户注册和登录流程，发放 JWT 令牌。
- **个人信息管理**: 查看和更新个人资料（头像、简介等）。
- **安全**: 密码加密存储及修改密码功能。
- **角色权限**: 基础的角色支持（如普通用户、管理员）。

#### 🎵 音乐模块 (Music)
- **音乐管理**: 音乐的上传（创建）、更新、删除和详情获取。
- **搜索功能**: 支持按歌曲名称或歌手进行模糊搜索。
- **互动功能**: 收藏（喜欢）音乐与取消收藏。
- **用户列表**: 查看指定用户发布的音乐，以及查看指定用户收藏的音乐列表。
- **数据模型**: 支持单曲（Single）和专辑（Album）类型。

### 项目结构
```
music-online-go/
├── cmd/
│   └── server/          # 程序入口
├── internal/
│   ├── config/          # 配置加载
│   ├── domain/          # 领域模型 (Model) 和 DTO
│   ├── handler/         # HTTP 请求处理器 (Controller)
│   ├── middleware/      # Gin 中间件 (认证, 日志, CORS)
│   ├── pkg/             # 公共包 (数据库, JWT, 工具类)
│   ├── repository/      # 数据访问层 (DAO)
│   └── service/         # 业务逻辑层 (Service)
├── config-example.yaml  # 配置文件模板
├── Dockerfile           # Dockerfile 文件
├── README.md
└── LICENSE
```

### 快速开始

1. **前置要求**
   - 安装 Go 1.24 或更高版本
   - 安装并启动 PostgreSQL 数据库

2. **配置**
   将 `config-example.yaml` 复制为 `config.yaml` 并修改数据库连接信息：
   ```bash
   cp config-example.yaml config.yaml
   ```

3. **运行程序**
   ```bash
   go run cmd/server/main.go
   ```
   服务默认将在 `8080` 端口启动。
