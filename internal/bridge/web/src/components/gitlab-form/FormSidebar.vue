<script setup lang="ts">
export interface NavItem {
  id: string
  title: string
  hasError?: boolean
  isDirty?: boolean
  /** A short status word shown after the title, e.g. "On" or "Soon". */
  status?: string
  disabled?: boolean
}

export interface NavGroup {
  /** Omitted for the first, unlabeled group. */
  label?: string
  items: NavItem[]
}

defineProps<{
  groups: NavGroup[]
  activeId: string
}>()

const emit = defineEmits<{ select: [id: string] }>()
</script>

<template>
  <nav class="sidebar" aria-label="Configuration">
    <div v-for="(group, index) in groups" :key="group.label ?? index" class="nav-group">
      <p v-if="group.label" class="nav-group-label">{{ group.label }}</p>

      <button
        v-for="item in group.items"
        :key="item.id"
        type="button"
        class="nav-item"
        :class="{ 'nav-item--active': activeId === item.id }"
        :disabled="item.disabled"
        :title="item.disabled ? 'Not configurable yet' : undefined"
        :aria-current="activeId === item.id ? 'page' : undefined"
        @click="emit('select', item.id)"
      >
        <span>{{ item.title }}</span>
        <span class="nav-item-indicators">
          <span v-if="item.isDirty" class="nav-item-dot" title="Unsaved changes"></span>
          <span v-if="item.hasError" class="nav-item-badge" title="Needs attention"></span>
          <span v-if="item.status" class="nav-item-status">{{ item.status }}</span>
        </span>
      </button>
    </div>
  </nav>
</template>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  flex: 0 0 200px;
  position: sticky;
  top: 1rem;
}

.nav-group {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.nav-group-label {
  margin: 0 0 0.35rem 0.6rem;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  opacity: 0.6;
}

.nav-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.5rem 0.6rem;
  border: 1px solid transparent;
  border-radius: 4px;
  text-align: left;
  opacity: 0.75;
}

.nav-item--active {
  background: var(--color-background-mute);
  border-color: var(--color-border);
  opacity: 1;
  font-weight: 500;
}

.nav-item:disabled {
  opacity: 0.45;
  cursor: default;
}

.nav-item-indicators {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex: none;
}

.nav-item-badge {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  background: #d64541;
  flex: none;
}

/*
 * A plain dot in the text color rather than a color of its own: unlike the
 * error badge, this is not a severity, just a fact about the section, so it
 * should not compete with the error color for attention.
 */
.nav-item-dot {
  width: 0.4rem;
  height: 0.4rem;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.8;
  flex: none;
}

.nav-item-status {
  font-size: 0.75rem;
  opacity: 0.75;
}
</style>
