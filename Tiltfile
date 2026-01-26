# Tiltfile for @cli-tools
# Build system for Go CLI commands and shell scripts

# Configuration
config.define_string('deploy_bin', args=False)
cfg = config.parse()

DEPLOY_BIN = cfg.get('deploy_bin', os.path.expanduser('~/bin'))
CLI_BIN = os.path.abspath('./bin')
CMD_SRC = os.path.abspath('./src/go/cmd')

# =============================================================================
# Go Commands
# =============================================================================

# Discover all Go commands (directories with main.go)
go_cmd_dirs = str(local('find {} -name "main.go" -type f'.format(CMD_SRC), quiet=True)).strip().split('\n')
go_commands = [os.path.dirname(p).split('/')[-1] for p in go_cmd_dirs if p]

def go_build_cmd(name):
    """Generate build command for a Go binary"""
    src = '{}/{}/main.go'.format(CMD_SRC, name)
    dst = '{}/{}'.format(DEPLOY_BIN, name)
    return 'go build -o {} {}'.format(dst, src)

# Create a local resource for each Go command
for cmd in go_commands:
    local_resource(
        'go:{}'.format(cmd),
        cmd=go_build_cmd(cmd),
        deps=['{}/{}'.format(CMD_SRC, cmd)],
        labels=['go', 'build'],
        allow_parallel=True,
    )

# =============================================================================
# Shell Scripts (from bin/)
# =============================================================================

# Get shell scripts from bin/ (exclude .md, .backup, etc)
shell_scripts = str(local(
    'find {} -maxdepth 1 -type f ! -name "*.md" ! -name "*.backup" ! -name ".*"'.format(CLI_BIN),
    quiet=True
)).strip().split('\n')
shell_scripts = [os.path.basename(s) for s in shell_scripts if s]

def shell_deploy_cmd(name):
    """Generate deploy command for a shell script (symlink)"""
    src = '{}/{}'.format(CLI_BIN, name)
    dst = '{}/{}'.format(DEPLOY_BIN, name)
    return 'ln -sf {} {}'.format(src, dst)

# Create a local resource for deploying shell scripts
local_resource(
    'shell:deploy',
    cmd=' && '.join([shell_deploy_cmd(s) for s in shell_scripts]),
    deps=[CLI_BIN],
    labels=['shell', 'deploy'],
    resource_deps=[],
)

# =============================================================================
# Permissions
# =============================================================================

local_resource(
    'permissions',
    cmd='chmod +x {}/* ./scripts/* 2>/dev/null || true'.format(CLI_BIN),
    deps=[CLI_BIN, './scripts'],
    labels=['setup'],
    auto_init=True,
)

# =============================================================================
# Aggregate Resources
# =============================================================================

# Build all Go commands
local_resource(
    'build:go',
    cmd=' && '.join([go_build_cmd(cmd) for cmd in go_commands]) if go_commands else 'echo "No Go commands found"',
    deps=[CMD_SRC],
    labels=['build', 'all'],
    auto_init=False,
)

# Full build (Go + permissions)
local_resource(
    'build:all',
    cmd='echo "Build complete"',
    resource_deps=['build:go', 'permissions', 'shell:deploy'],
    labels=['build', 'all'],
    auto_init=False,
)

# =============================================================================
# Development helpers
# =============================================================================

# Watch and rebuild on changes
local_resource(
    'watch:go',
    serve_cmd='while true; do find {} -name "*.go" | entr -d task go:build; done'.format(CMD_SRC),
    labels=['dev'],
    auto_init=False,
)

# =============================================================================
# Info
# =============================================================================

print('=' * 60)
print('CLI Tools Build System')
print('=' * 60)
print('Deploy target: {}'.format(DEPLOY_BIN))
print('Go commands:   {}'.format(', '.join(go_commands) if go_commands else 'none'))
print('Shell scripts: {}'.format(len(shell_scripts)))
print('=' * 60)
print('')
print('Resources:')
print('  build:all     - Build everything')
print('  build:go      - Build all Go commands')
print('  shell:deploy  - Deploy shell scripts')
print('  permissions   - Set executable permissions')
print('  go:<name>     - Build individual Go command')
print('')
