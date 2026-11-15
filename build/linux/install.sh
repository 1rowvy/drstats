#!/bin/sh
# Installs drstats for the current user (no root): binary in ~/.local/bin,
# menu entry and icon in ~/.local/share. A per-user install keeps the
# built-in updater working, since it replaces the binary in place.
#
#   ./install.sh              install or update
#   ./install.sh --uninstall  remove (your stats in ~/.config/drstats are kept)
set -eu

APP_ID=io.github.drstats
here=$(cd "$(dirname "$0")" && pwd)
bin_dir=${XDG_BIN_HOME:-$HOME/.local/bin}
data_dir=${XDG_DATA_HOME:-$HOME/.local/share}
config_dir=${XDG_CONFIG_HOME:-$HOME/.config}
desktop="$data_dir/applications/$APP_ID.desktop"
icon="$data_dir/icons/hicolor/256x256/apps/$APP_ID.png"

refresh() {
	command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q "$data_dir/applications" || true
}

if [ "${1:-}" = "--uninstall" ]; then
	rm -f "$bin_dir/drstats" "$desktop" "$icon" "$config_dir/autostart/drstats.desktop"
	refresh
	echo "drstats removed. Your stats are kept in $config_dir/drstats."
	exit 0
fi

mkdir -p "$bin_dir" "$(dirname "$desktop")" "$(dirname "$icon")"
install -m 755 "$here/drstats" "$bin_dir/drstats"
install -m 644 "$here/drstats.png" "$icon"
cat > "$desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=drstats
Comment=Dota 2 replay stats
Exec="$bin_dir/drstats"
Icon=$APP_ID
StartupWMClass=$APP_ID
Categories=Game;
Keywords=dota;replay;stats;
Terminal=false
DESKTOP
refresh
echo "drstats installed: $bin_dir/drstats (menu: drstats)."
case ":$PATH:" in
*":$bin_dir:"*) ;;
*) echo "Note: $bin_dir is not in PATH; start drstats from the app menu." ;;
esac
