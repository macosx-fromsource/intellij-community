import { createRouter, createWebHistory } from 'vue-router'

import GitLabsListView from '@/views/GitLabsListView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'gitlabs',
      component: GitLabsListView,
    },
    {
      path: '/gitlabs/new',
      name: 'gitlab-create',
      component: () => import('@/views/GitLabFormView.vue'),
    },
    {
      path: '/gitlabs/:namespace/:name/edit',
      name: 'gitlab-edit',
      component: () => import('@/views/GitLabFormView.vue'),
      props: true,
    },
  ],
})

export default router
