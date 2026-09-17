<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ phase?: string }>()

const label = computed(() => props.phase || 'Unknown')

const variant = computed(() => {
  switch (props.phase) {
    case 'Running':
      return 'running'
    case 'Upgrading':
      return 'upgrading'
    case 'Failed':
      return 'failed'
    case 'Preparing':
      return 'preparing'
    default:
      return 'unknown'
  }
})
</script>

<template>
  <span class="badge" :class="`badge--${variant}`">
    <span class="dot" aria-hidden="true"></span>
    {{ label }}
  </span>
</template>

<style scoped>
.badge {
  --badge: var(--vt-c-text-light-2);

  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.1rem 0.55rem;
  border: 1px solid color-mix(in srgb, var(--badge) 40%, transparent);
  border-radius: 999px;
  background: color-mix(in srgb, var(--badge) 14%, transparent);
  color: var(--badge);
  font-size: 0.8rem;
  font-weight: 500;
  white-space: nowrap;
}

.dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  background: var(--badge);
}

/* Fixed, mid-tone hues so both the tint and the text stay legible on the
   light and dark themes alike. */
.badge--running {
  --badge: #3fb950;
}

.badge--upgrading {
  --badge: #58a6ff;
}

.badge--failed {
  --badge: #f85149;
}

.badge--preparing {
  --badge: #d29922;
}

.badge--unknown {
  --badge: #8b949e;
}

/* The dot pulses while an upgrade is in flight. */
.badge--upgrading .dot {
  animation: pulse 1.4s ease-in-out infinite;
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }

  50% {
    opacity: 0.3;
  }
}

@media (prefers-reduced-motion: reduce) {
  .badge--upgrading .dot {
    animation: none;
  }
}
</style>
