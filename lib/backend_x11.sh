#!/usr/bin/env bash

# Remote Studio - X11 Backend Abstraction
# This encapsulates all display and hardware interactions that are specific to the X Window System.

backend_get_active_display() {
    xrandr 2>/dev/null | awk '/ connected/ {out=$1} /\*/ {print out " " $1; exit}'
}

backend_get_connected_outputs() {
    xrandr 2>/dev/null | grep " connected" | cut -f1 -d" "
}

backend_get_current_active_mode() {
    xrandr 2>/dev/null | grep '\*' | head -n 1 | awk '{print $1}'
}

backend_apply_native_mode() {
    local output="$1"
    local target_res="$2"
    xrandr --output "$output" --mode "$target_res" 2>/dev/null
}

backend_apply_custom_mode() {
    local output="$1"
    local mode_name="$2"
    local width="$3"
    local height="$4"
    local freq="${5:-60}"

    if xrandr --output "$output" --mode "$mode_name" 2>/dev/null; then
        return 0
    fi

    local mode_info mode_params xrandr_err
    mode_info=$(cvt "$width" "$height" "$freq" | grep Modeline)
    mode_params=$(echo "$mode_info" | cut -d' ' -f3-)
    
    # shellcheck disable=SC2086
    xrandr --newmode "$mode_name" $mode_params 2>/dev/null || true
    xrandr --addmode "$output" "$mode_name" 2>/dev/null || true

    xrandr_err=$(xrandr --output "$output" --mode "$mode_name" 2>&1) || {
        echo "Error: could not apply mode $mode_name: $xrandr_err" >&2
        return 1
    }
    return 0
}

backend_rotate_display() {
    local output="$1"
    local dir="$2"
    xrandr --output "$output" --rotate "$dir"
}

backend_get_current_rotation() {
    xrandr 2>/dev/null | grep " connected" | grep -o "normal\|left\|right\|inverted" | head -1 || echo "normal"
}

backend_set_dpi() {
    local dpi="$1"
    echo "Xft.dpi: $dpi" | xrdb -merge 2>/dev/null || true
}

backend_get_gamma_state() {
    local gamma
    gamma=$(xgamma 2>&1 | awk '{print $4}')
    if [[ "$gamma" == "1.000" ]]; then
        echo "OFF"
    else
        echo "ON"
    fi
}

backend_toggle_night_shift() {
    local gamma
    gamma=$(xgamma 2>&1 | awk '{print $4}')
    if [[ "$gamma" == "1.000" ]]; then
        xgamma -rgamma 1.0 -ggamma 0.8 -bgamma 0.6 >/dev/null 2>&1
        echo "ON"
    else
        xgamma -gamma 1.0 >/dev/null 2>&1
        echo "OFF"
    fi
}

backend_clear_clipboard() {
    echo -n "" | xclip -selection primary 2>/dev/null || true
    echo -n "" | xclip -selection clipboard 2>/dev/null || true
}

backend_copy_to_clipboard() {
    echo -n "$1" | xclip -selection clipboard 2>/dev/null || true
}
