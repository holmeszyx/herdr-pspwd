__herdr_osc7() {
  local path out='' c n i
  path=$(cygpath -m "$PWD")
  for (( i = 0; i < ${#path}; i++ )); do
    c=${path:i:1}
    case "$c" in
      [-/:._~a-zA-Z0-9]) out+=$c ;;
      *)
        printf -v n '%d' "'$c"
        if (( n < 128 )); then
          printf -v c '%%%02X' "$n"
        fi
        out+=$c
        ;;
    esac
  done
  printf '\e]7;file:///%s\a' "$out"
}
if [[ "$HERDR_ENV" == "1" && "$PROMPT_COMMAND" != *__herdr_osc7* ]]; then
  PROMPT_COMMAND="__herdr_osc7${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
