#!/usr/bin/env bash
# shellcheck disable=SC2034 # Callers use different fields from the shared app descriptor.

load_nox_app() {
  local app="$1"
  case "$app" in
    player)
      deploy_git_dir="${KINOSAIL_DEPLOY_GIT_DIR:-}"
      nox_host="${KINOSAIL_NOX_HOST:-nox@server.nox}"
      app_path=apps/player
      image_repo=localhost/kinosail
      container=kinosail
      service=kinosail
      archive_shared=1
      deploy_success='Deployed and verified Nox at'
      watch_root="${KINOSAIL_DEPLOY_ROOT:-$HOME/Library/Caches/KinosailDeploy}"
      watch_repo_url="${KINOSAIL_DEPLOY_REPO_URL:-https://github.com/MikeO7/kinosail.git}"
      repo_variable=KINOSAIL_DEPLOY_REPO_URL
      watch_interval="${KINOSAIL_DEPLOY_INTERVAL:-10}"
      git_dir_name=KINOSAIL_DEPLOY_GIT_DIR
      install_root="$HOME/Library/Application Support/KinosailDeploy"
      install_cache="$HOME/Library/Caches/KinosailDeploy"
      launch_label=com.kinosail.deploy-nox
      root_variable=KINOSAIL_DEPLOY_ROOT
      install_success='Nox auto-deploy installed; polling origin/main every 10 seconds'
      ;;
    subtitles)
      deploy_git_dir="${KINOSAIL_SUBTITLES_DEPLOY_GIT_DIR:-}"
      nox_host="${KINOSAIL_SUBTITLES_NOX_HOST:-nox@server.nox}"
      app_path=apps/subtitles
      image_repo=localhost/kinosail-subtitles
      container=kinosail-subtitles-dev
      service=kinosail-subtitles
      archive_shared=1
      deploy_success='Deployed and verified Kinosail Subtitles on Nox at'
      watch_root="${KINOSAIL_SUBTITLES_DEPLOY_ROOT:-$HOME/Library/Caches/KinosailSubtitlesDeploy}"
      watch_repo_url="${KINOSAIL_SUBTITLES_DEPLOY_REPO_URL:-https://github.com/MikeO7/kinosail.git}"
      repo_variable=KINOSAIL_SUBTITLES_DEPLOY_REPO_URL
      watch_interval="${KINOSAIL_SUBTITLES_DEPLOY_INTERVAL:-10}"
      git_dir_name=KINOSAIL_SUBTITLES_DEPLOY_GIT_DIR
      install_root="$HOME/Library/Application Support/KinosailSubtitlesDeploy"
      install_cache="$HOME/Library/Caches/KinosailSubtitlesDeploy"
      launch_label=com.kinosail.subtitles.deploy-nox
      root_variable=KINOSAIL_SUBTITLES_DEPLOY_ROOT
      install_success='Kinosail Subtitles Nox auto-deploy installed; polling origin/main every 10 seconds'
      ;;
    dashboard)
      deploy_git_dir="${KINOSAIL_DASHBOARD_DEPLOY_GIT_DIR:-}"
      nox_host="${KINOSAIL_DASHBOARD_NOX_HOST:-nox@server.nox}"
      app_path=apps/dashboard
      image_repo=localhost/kinosail-dashboard
      container=kinosail-dashboard
      service=kinosail-dashboard
      archive_shared=1
      deploy_success='Deployed and verified Kinosail Dashboard on Nox at'
      watch_root="${KINOSAIL_DASHBOARD_DEPLOY_ROOT:-$HOME/Library/Caches/KinosailDashboardDeploy}"
      watch_repo_url="${KINOSAIL_DASHBOARD_DEPLOY_REPO_URL:-https://github.com/MikeO7/kinosail.git}"
      repo_variable=KINOSAIL_DASHBOARD_DEPLOY_REPO_URL
      watch_interval="${KINOSAIL_DASHBOARD_DEPLOY_INTERVAL:-10}"
      git_dir_name=KINOSAIL_DASHBOARD_DEPLOY_GIT_DIR
      install_root="$HOME/Library/Application Support/KinosailDashboardDeploy"
      install_cache="$HOME/Library/Caches/KinosailDashboardDeploy"
      launch_label=com.kinosail.dashboard.deploy-nox
      root_variable=KINOSAIL_DASHBOARD_DEPLOY_ROOT
      install_success='Kinosail Dashboard Nox auto-deploy installed; polling origin/main every 10 seconds'
      ;;
    *)
      printf 'unknown Kinosail app: %s\n' "$app" >&2
      return 2
      ;;
  esac
}
