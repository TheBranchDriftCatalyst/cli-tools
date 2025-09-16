#!/bin/zsh

# TODO: get or set the catalyst worktools environment variable, or use default, also warn if not set 
# TODO: also use fzf and sexify this

function work_on() {
    local workspace_dir=".workspaces"
    local workspace_file

    echo "Searching for workspace files in ${workspace_dir}..."

    workspace_file=$(find "${workspace_dir}" -name '*.code-workspace' | fzf --prompt='Select workspace: ')

    if [[ -n "$workspace_file" ]]; then
        echo "Opening $workspace_file..."
        code "$workspace_file"
    else
        echo "No workspace selected."
    fi
}