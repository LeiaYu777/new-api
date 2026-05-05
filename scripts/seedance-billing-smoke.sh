#!/usr/bin/env bash
set -euo pipefail

# Smoke test for the Seedance 2.0 billing path:
# API key quota/subscription -> submit video task -> poll task result.
#
# Required:
#   API_KEY=sk-...
#
# Optional:
#   BASE_URL=http://localhost:3000
#   MODEL=doubao-seedance-2-0
#   PROMPT="..."
#   DURATION=5
#   RESOLUTION=720p
#   RATIO=16:9
#   GENERATE_AUDIO=false
#   IMAGE_URL=https://cdn.example.com/reference.png
#   REFERENCE_IMAGE_URL=https://cdn.example.com/style.png
#   FIRST_FRAME_URL=https://cdn.example.com/first.png
#   LAST_FRAME_URL=https://cdn.example.com/last.png
#   REFERENCE_VIDEO_URL=https://cdn.example.com/reference.mp4
#   CALLBACK_URL=https://app.example.com/seedance/callback
#   EXPECT_SUBMIT_FAILURE=false
#   POLL_INTERVAL=5
#   POLL_ATTEMPTS=36

BASE_URL="${BASE_URL:-http://localhost:3000}"
MODEL="${MODEL:-doubao-seedance-2-0}"
PROMPT="${PROMPT:-A cinematic product video of a futuristic SaaS dashboard, smooth camera motion}"
DURATION="${DURATION:-5}"
RESOLUTION="${RESOLUTION:-720p}"
RATIO="${RATIO:-16:9}"
GENERATE_AUDIO="${GENERATE_AUDIO:-false}"
IMAGE_URL="${IMAGE_URL:-}"
REFERENCE_IMAGE_URL="${REFERENCE_IMAGE_URL:-}"
FIRST_FRAME_URL="${FIRST_FRAME_URL:-}"
LAST_FRAME_URL="${LAST_FRAME_URL:-}"
REFERENCE_VIDEO_URL="${REFERENCE_VIDEO_URL:-}"
CALLBACK_URL="${CALLBACK_URL:-}"
EXPECT_SUBMIT_FAILURE="${EXPECT_SUBMIT_FAILURE:-false}"
POLL_INTERVAL="${POLL_INTERVAL:-5}"
POLL_ATTEMPTS="${POLL_ATTEMPTS:-36}"

if [[ -z "${API_KEY:-}" ]]; then
  echo "API_KEY is required, for example: API_KEY=sk-xxx $0" >&2
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for this smoke script" >&2
  exit 2
fi

submit_payload="$(
  jq -n \
    --arg model "$MODEL" \
    --arg prompt "$PROMPT" \
    --arg resolution "$RESOLUTION" \
    --arg ratio "$RATIO" \
    --arg image_url "$IMAGE_URL" \
    --arg reference_image_url "$REFERENCE_IMAGE_URL" \
    --arg first_frame_url "$FIRST_FRAME_URL" \
    --arg last_frame_url "$LAST_FRAME_URL" \
    --arg reference_video_url "$REFERENCE_VIDEO_URL" \
    --arg callback_url "$CALLBACK_URL" \
    --argjson duration "$DURATION" \
    --argjson generate_audio "$GENERATE_AUDIO" \
    '{
      model: $model,
      prompt: $prompt,
      duration: $duration,
      resolution: $resolution,
      ratio: $ratio,
      generate_audio: $generate_audio
    }
    | if $image_url != "" then . + {images: [$image_url]} else . end
    | if $reference_image_url != "" then . + {reference_image_url: $reference_image_url} else . end
    | if $first_frame_url != "" then . + {first_frame_url: $first_frame_url} else . end
    | if $last_frame_url != "" then . + {last_frame_url: $last_frame_url} else . end
    | if $reference_video_url != "" then . + {reference_video_url: $reference_video_url} else . end
    | if $callback_url != "" then . + {callback_url: $callback_url} else . end'
)"

echo "Submitting Seedance 2.0 task to ${BASE_URL}/v1/video/generations"
submit_http_response="$(
  curl -sS \
    -w $'\n%{http_code}' \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$submit_payload" \
    "${BASE_URL}/v1/video/generations"
)"
submit_status="${submit_http_response##*$'\n'}"
submit_response="${submit_http_response%$'\n'*}"
echo "$submit_response" | jq . || echo "$submit_response"

if [[ "$EXPECT_SUBMIT_FAILURE" == "true" ]]; then
  if [[ "$submit_status" =~ ^[45][0-9][0-9]$ ]]; then
    echo "Submit failed as expected with HTTP ${submit_status}."
    exit 0
  fi
  echo "Expected submit failure, got HTTP ${submit_status}." >&2
  exit 1
fi

if [[ ! "$submit_status" =~ ^2[0-9][0-9]$ ]]; then
  echo "Submit failed with HTTP ${submit_status}." >&2
  exit 1
fi

task_id="$(echo "$submit_response" | jq -r '.task_id // .id // empty')"
if [[ -z "$task_id" || "$task_id" == "null" ]]; then
  echo "No task_id found in submit response" >&2
  exit 1
fi

echo "Polling task: ${task_id}"
for ((i = 1; i <= POLL_ATTEMPTS; i++)); do
  poll_response="$(
    curl -fsS \
      -H "Authorization: Bearer ${API_KEY}" \
      "${BASE_URL}/v1/video/generations/${task_id}"
  )"
  status="$(echo "$poll_response" | jq -r '.status // .data.status // empty')"
  echo "Attempt ${i}/${POLL_ATTEMPTS}: status=${status:-unknown}"
  echo "$poll_response" | jq .

  case "$status" in
    completed | succeeded | SUCCESS)
      echo "Seedance task completed. Check /console/billing for consume/refund/net quota."
      exit 0
      ;;
    failed | FAILURE)
      echo "Seedance task failed. Check /console/billing for refund entry."
      exit 1
      ;;
  esac

  sleep "$POLL_INTERVAL"
done

echo "Task did not finish within polling window. Check task logs and /console/billing later."
exit 1
