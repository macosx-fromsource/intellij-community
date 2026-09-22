<script setup lang="ts">
import { computed, onMounted, ref, useTemplateRef } from 'vue'
import { useRouter } from 'vue-router'

import '@/assets/gitlab-form.css'
import BasicsSection from '@/components/gitlab-form/BasicsSection.vue'
import DependenciesSection from '@/components/gitlab-form/DependenciesSection.vue'
import FormSidebar, { type NavGroup } from '@/components/gitlab-form/FormSidebar.vue'
import OpenBaoSection from '@/components/gitlab-form/OpenBaoSection.vue'
import OverridesSection from '@/components/gitlab-form/OverridesSection.vue'
import type { GitLabResource } from '@/lib/api/client'
import { useGitLabsStore } from '@/stores/gitlabs'

const props = defineProps<{
  namespace?: string
  name?: string
}>()

const store = useGitLabsStore()
const router = useRouter()

const isEdit = Boolean(props.namespace && props.name)

/**
 * Where a new instance is created. The Operator installation owns this
 * namespace, so the form does not offer a choice; an existing resource is
 * edited wherever it already lives.
 */
const defaultNamespace = 'gitlab-system'

const namespace = ref(props.namespace ?? defaultNamespace)

/** A section of the form, addressed by the sidebar. */
type SectionId = 'basics' | 'dependencies' | 'overrides' | 'openbao'

const activeSection = ref<SectionId>('basics')

/**
 * The subtitle shown above each section's fields. OpenBao has none: its
 * description already lives beneath its own checkbox (`OpenBaoSection.vue`),
 * so a subtitle up here would just repeat it.
 */
const summaries: Record<SectionId, string> = {
  basics: 'What the instance is, and which chart deploys it.',
  dependencies: 'The data stores the instance connects to.',
  openbao: '',
  overrides: 'The chart values, for everything the sections above do not cover.',
}

const currentSummary = computed(() => summaries[activeSection.value])

const basics = useTemplateRef<InstanceType<typeof BasicsSection>>('basics')
const dependencies = useTemplateRef<InstanceType<typeof DependenciesSection>>('dependencies')
const openbao = useTemplateRef<InstanceType<typeof OpenBaoSection>>('openbao')
const overrides = useTemplateRef<InstanceType<typeof OverridesSection>>('overrides')

/**
 * Add-ons with no panel yet. They are listed so the sidebar shows the full
 * set of GitLab subcomponents the Operator is growing toward, but each is a
 * disabled placeholder until it has fields of its own to configure.
 */
const plannedAddons = [
  { id: 'siphon', title: 'Siphon' },
  { id: 'orbit', title: 'Orbit' },
  { id: 'artifact-registry', title: 'Artifact Registry' },
  { id: 'ai-gateway', title: 'AI Gateway' },
] as const

/**
 * The sidebar, grouped as the core instance configuration (always present),
 * **Add-ons** (GitLab subcomponents an instance can optionally turn on), and
 * **Advanced** (settings most instances leave alone). Each item's
 * error/dirty/enabled state is read from the section component itself,
 * through the template refs above.
 */
const navGroups = computed<NavGroup[]>(() => [
  {
    items: [
      { id: 'basics', title: 'Basics', hasError: basics.value?.hasError, isDirty: basics.value?.isDirty },
      {
        id: 'dependencies',
        title: 'Dependencies',
        hasError: dependencies.value?.hasError,
        isDirty: dependencies.value?.isDirty,
      },
    ],
  },
  {
    label: 'Add-ons',
    items: [
      {
        id: 'openbao',
        title: 'Secret Manager',
        hasError: openbao.value?.hasError,
        isDirty: openbao.value?.isDirty,
        status: openbao.value?.enabled ? 'On' : undefined,
      },
      ...plannedAddons.map((entry) => ({ id: entry.id, title: entry.title, status: 'Soon', disabled: true })),
    ].sort((a, b) => a.title.localeCompare(b.title)),
  },
  {
    label: 'Advanced',
    items: [
      {
        id: 'overrides',
        title: 'Overrides',
        hasError: overrides.value?.hasError,
        isDirty: overrides.value?.isDirty,
      },
    ],
  },
])

onMounted(async () => {
  if (isEdit) {
    const ok = await store.fetchOne(props.namespace!, props.name!)
    if (ok && store.current) {
      basics.value?.loadFrom(store.current)
      dependencies.value?.loadFrom(store.current)
      openbao.value?.loadFrom(store.current)
      overrides.value?.loadFrom(store.current)
    }
  }

  // Taken once the fetched values (or a new instance's defaults) have
  // settled, so editing what was just loaded is what "unsaved" means, not
  // loading it.
  basics.value?.resetBaseline()
  dependencies.value?.resetBaseline()
  openbao.value?.resetBaseline()
  overrides.value?.resetBaseline()
})

async function submit() {
  const order: { id: SectionId; section: { validate: () => boolean } | null }[] = [
    { id: 'basics', section: basics.value },
    { id: 'dependencies', section: dependencies.value },
    { id: 'openbao', section: openbao.value },
    { id: 'overrides', section: overrides.value },
  ]

  for (const { id, section } of order) {
    if (!section?.validate()) {
      activeSection.value = id

      return
    }
  }

  const basicsData = basics.value!.toPartial()

  const resource: GitLabResource = {
    name: basicsData.name,
    namespace: namespace.value,
    hostname: basicsData.hostname,
    edition: basicsData.edition,
    license: basicsData.license,
    ...dependencies.value!.toPartial(),
    openbao: openbao.value!.toPartial(),
    chart: {
      version: basicsData.version,
      values: overrides.value!.toPartial(),
    },
  }

  const ok = isEdit
    ? await store.update(namespace.value, resource.name, resource)
    : await store.create(resource)

  if (ok) {
    basics.value!.resetBaseline()
    dependencies.value!.resetBaseline()
    openbao.value!.resetBaseline()
    overrides.value!.resetBaseline()
    router.push({ name: 'gitlabs' })
  }
}
</script>

<template>
  <section>
    <h1>{{ isEdit ? 'Edit GitLab instance' : 'New GitLab instance' }}</h1>

    <p v-if="store.error" class="error" role="alert">{{ store.error }}</p>

    <form class="editor gitlab-form" @submit.prevent="submit">
      <FormSidebar :groups="navGroups" :active-id="activeSection" @select="activeSection = $event as SectionId" />

      <div class="content">
        <p v-if="currentSummary" class="hint section-summary">{{ currentSummary }}</p>

        <!--
          v-show rather than v-if: the values editor inside OverridesSection
          is a CodeMirror instance that mounts once, and every section keeps
          its state while another one is shown.
        -->
        <BasicsSection
          v-show="activeSection === 'basics'"
          ref="basics"
          :namespace="namespace"
          :is-edit="isEdit"
          :initial-name="props.name ?? 'gitlab'"
        />

        <DependenciesSection v-show="activeSection === 'dependencies'" ref="dependencies" />

        <OverridesSection v-show="activeSection === 'overrides'" ref="overrides" />

        <OpenBaoSection v-show="activeSection === 'openbao'" ref="openbao" />

        <div class="actions">
          <button type="submit" :disabled="store.loading">
            {{ isEdit ? 'Save' : 'Create' }}
          </button>
          <button type="button" @click="router.push({ name: 'gitlabs' })">Cancel</button>
        </div>
      </div>
    </form>
  </section>
</template>

<style scoped>
.editor {
  display: flex;
  align-items: flex-start;
  gap: 2rem;
  margin-top: 1rem;
}

.content {
  flex: 1 1 auto;
  min-width: 0;
  max-width: 640px;
}

.section-summary {
  margin: 0 0 1rem;
}

.actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.error {
  color: #d64541;
  white-space: pre-wrap;
}
</style>
