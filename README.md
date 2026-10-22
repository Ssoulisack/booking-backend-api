# 🧱 Go Backend Template 2026

A modern, scalable backend template built with Go. Designed for rapid development and clean architecture in 2025.

---

## 🚀 Getting Started

Follow these steps to clone and set up your own project based on this template.

### 📥 STEP 1 — Clone the Repository

```bash
git clone <your-template-repo-url> your-project-name
cd your-project-name
```

### 🧹 STEP 2 — Remove Existing Git History

```bash
git remote remove origin
rm -rf .git
git init
```

### 🔗 STEP 3 — Add Your Own Git Repository

```bash
git remote add origin <your-own-repo-url>
git add .
git commit -m "Initial commit from Go Backend Template"
git push -u origin main
```

### 🛠️ Run the Application

```bash
make run
```


### 📦 Project Structure
```
    .
    ├── cmd/                # Main application entry points
    ├── internal/           # Private application and business logic
    │   ├── primary/        # HTTP handlers
    │   ├── secondary/     # Database access
    ├── pkg/                # Shared packages (utilities, helpers)
    ├── .env                # Environment variables
    ├── Makefile            # Build and run commands
    ├── go.mod              # Go module definition
    └── README.md           # Project documentation
```# go-backend-template
