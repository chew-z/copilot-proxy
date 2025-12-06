# Environment Loading with godotenv/autoload

We use `github.com/joho/godotenv/autoload` to automatically load environment variables from a local `.env` file during development and tests. The side-effect import keeps the main code paths clean while ensuring config is populated early.

## Usage

- Add the side-effect import once in the program entry point (e.g. `main.go` or CLI root):  
  `import _ "github.com/joho/godotenv/autoload"`
- Keep the `.env` file at the project root; the loader picks it up automatically.
- Treat `.env` as dev-only. Production should rely on real environment variables or secret managers.

## Notes

- Avoid multiple imports; a single global autoload is sufficient.
- Do not commit secrets in `.env`. Add it to `.gitignore` if not already ignored.
- If `.env` is absent, autoload is a no-op, so CI/CD remains unaffected.
