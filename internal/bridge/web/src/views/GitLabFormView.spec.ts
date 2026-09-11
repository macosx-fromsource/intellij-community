// @vitest-environment jsdom
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import { api } from '@/lib/api/client'
import GitLabFormView from './GitLabFormView.vue'

vi.mock('@/lib/api/client', () => ({
  api: {
    GET: vi.fn<() => Promise<unknown>>(),
    POST: vi.fn<() => Promise<unknown>>(),
    PUT: vi.fn<() => Promise<unknown>>(),
    DELETE: vi.fn<() => Promise<unknown>>(),
  },
  errorMessage: (error: unknown) => (error as { detail?: string })?.detail ?? 'error',
}))

// The values editor is a CodeMirror instance, which jsdom cannot lay out. The
// step logic does not depend on it, so it is stubbed down to its model value.
vi.mock('@/components/YamlEditor.vue', () => ({
  default: {
    name: 'YamlEditor',
    props: ['modelValue'],
    template: '<textarea :value="modelValue" />',
  },
}))

const mockApi = api as unknown as { GET: Mock; POST: Mock }

/** Mounts the form and lets its mounted hook settle. */
async function mountForm(): Promise<VueWrapper> {
  const wrapper = mount(GitLabFormView)
  await flushPromises()

  return wrapper
}

const push = vi.fn<(to: unknown) => void>()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

/** Titles of the steps, in the order the header shows them. */
const stepTitles = ['Basics', 'Dependencies', 'Overrides']

/** Returns the title of the step the form is on. */
function currentStep(wrapper: VueWrapper): string {
  return wrapper
    .get('[aria-current="step"]')
    .text()
    .replace(/^\d+\s*/, '')
}

/** Selects a step in the header. */
async function selectStep(wrapper: VueWrapper, title: string) {
  const button = wrapper.findAll('nav button').find((candidate) => candidate.text().includes(title))

  await button!.trigger('click')
}

/**
 * The panel of the step that is shown. The others stay in the DOM behind
 * `v-show`, so a query has to be scoped to this one to reach the right field.
 */
function panel(wrapper: VueWrapper) {
  return wrapper
    .findAll('.step-panel')
    .find((candidate) => (candidate.element as HTMLElement).style.display !== 'none')!
}

/** The fieldset of the named group within the visible step. */
function group(wrapper: VueWrapper, legend: string) {
  return panel(wrapper)
    .findAll('fieldset')
    .find((candidate) => candidate.get('legend').text().includes(legend))!
}

/** Fills an input by the label it sits under, within the visible step. */
async function fill(wrapper: VueWrapper, label: string, value: string) {
  const field = panel(wrapper)
    .findAll('label')
    .find((candidate) => candidate.text().startsWith(label))

  await field!.get('input').setValue(value)
}

describe('GitLabFormView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('starts on the first step and shows all three', async () => {
    const wrapper = await mountForm()

    expect(
      wrapper.findAll('nav button').map((button) => button.text().replace(/^\d+\s*/, '')),
    ).toEqual(stepTitles)
    expect(currentStep(wrapper)).toBe('Basics')
  })

  it('walks forward through the steps with Next', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')

    const next = () =>
      wrapper
        .findAll('.actions button')
        .find((button) => button.text() === 'Next')!
        .trigger('click')

    await next()
    expect(currentStep(wrapper)).toBe('Dependencies')

    await next()
    expect(currentStep(wrapper)).toBe('Overrides')
    expect(wrapper.findAll('.actions button').map((button) => button.text())).toContain('Create')
  })

  // The name identifies the resource, so an empty one holds the form on the step
  // it belongs to rather than reaching the API.
  it('keeps an incomplete first step from being left', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Name', '')
    await selectStep(wrapper, 'Overrides')

    expect(currentStep(wrapper)).toBe('Basics')
    expect(wrapper.text()).toContain('A name is required.')
  })

  // The namespace is the one the Operator installation owns, and the form offers
  // no choice of it.
  it('creates in gitlab-system without asking', async () => {
    const wrapper = await mountForm()

    const labels = panel(wrapper)
      .findAll('label span:first-child')
      .map((label) => label.text())

    expect(labels).not.toContain('Namespace')
    expect(panel(wrapper).text()).toContain('Created in the gitlab-system namespace.')
  })

  // Nothing about which versions the Operator can render is known here, so the
  // field starts empty rather than guessing.
  it('leaves the chart version empty by default', async () => {
    const wrapper = await mountForm()

    const input = panel(wrapper).get('input[name="chart-version"]')

    expect((input.element as HTMLInputElement).value).toBe('')
    expect(mockApi.GET).not.toHaveBeenCalled()
  })

  // The field is free-form: what renders is whatever the Operator bundles or
  // can pull, which the SPA has no way to know.
  it('takes any version', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '11.0.0-rc1')
    await selectStep(wrapper, 'Dependencies')

    expect(currentStep(wrapper)).toBe('Dependencies')
    expect(wrapper.text()).not.toContain('A chart version is required.')
  })

  // Nothing renders without a version, so the field cannot be cleared.
  it('requires a version', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '  ')
    await selectStep(wrapper, 'Dependencies')

    expect(currentStep(wrapper)).toBe('Basics')
    expect(wrapper.text()).toContain('A chart version is required.')
  })

  // A data store connection is all three fields or none, and the check belongs
  // to the step that holds them.
  it('reports an incomplete dependency on the step that owns it', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await selectStep(wrapper, 'Dependencies')
    await fill(wrapper, 'Hostname', 'gitlab-postgresql')
    await selectStep(wrapper, 'Overrides')

    expect(currentStep(wrapper)).toBe('Dependencies')
    expect(wrapper.text()).toContain('PostgreSQL needs a hostname')
  })

  // Object storage is one Secret rather than a host and a Secret, and it is
  // still all or nothing.
  it('reports an incomplete object storage connection', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await selectStep(wrapper, 'Dependencies')
    await group(wrapper, 'Object storage').findAll('input')[0]!.setValue('gitlab-object-storage')
    await selectStep(wrapper, 'Overrides')

    expect(currentStep(wrapper)).toBe('Dependencies')
    expect(wrapper.text()).toContain('Object storage needs both a Secret name and a Secret key.')
  })

  it('creates the resource from every step once the last one submits', async () => {
    mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })

    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await fill(wrapper, 'Hostname', 'gitlab.example.com')
    await selectStep(wrapper, 'Dependencies')

    const psql = group(wrapper, 'PostgreSQL').findAll('input')
    await psql[0]!.setValue('gitlab-postgresql')
    await psql[1]!.setValue('gitlab-postgresql-password')
    await psql[2]!.setValue('password')

    const valkey = group(wrapper, 'Valkey').findAll('input')
    await valkey[0]!.setValue('gitlab-valkey')
    await valkey[1]!.setValue('gitlab-valkey-auth')
    await valkey[2]!.setValue('default')

    const storage = group(wrapper, 'Object storage').findAll('input')
    await storage[0]!.setValue('gitlab-object-storage')
    await storage[1]!.setValue('connection')

    await selectStep(wrapper, 'Overrides')
    await wrapper.get('form').trigger('submit')

    expect(mockApi.POST).toHaveBeenCalledWith('/api/v1/namespaces/{namespace}/gitlabs', {
      params: { path: { namespace: 'gitlab-system' } },
      body: {
        name: 'gitlab',
        namespace: 'gitlab-system',
        hostname: 'gitlab.example.com',
        edition: 'ee',
        license: undefined,
        postgresql: {
          host: 'gitlab-postgresql',
          passwordSecretRef: { name: 'gitlab-postgresql-password', key: 'password' },
        },
        redis: {
          host: 'gitlab-valkey',
          passwordSecretRef: { name: 'gitlab-valkey-auth', key: 'default' },
        },
        objectStorage: {
          connectionSecretRef: { name: 'gitlab-object-storage', key: 'connection' },
        },
        chart: { version: '10.2.2', values: {} },
      },
    })
  })

  // Enter in a field submits a form, and the submit button lives on the last
  // step. Advancing is what that has to mean on the earlier ones.
  it('advances rather than creates when an earlier step submits', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await wrapper.get('form').trigger('submit')

    expect(currentStep(wrapper)).toBe('Dependencies')
    expect(mockApi.POST).not.toHaveBeenCalled()
  })
})
