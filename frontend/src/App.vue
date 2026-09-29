<script setup>
import { onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AppNav from '@/components/AppNav.vue'
import { useTelegram } from '@/composables/useTelegram'
import { useSessionStore } from '@/stores/session'

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const { showBackButton } = useTelegram()

onMounted(() => session.load())

// Deep links have no in-app history to go back to.
function goBack() {
  if (window.history.state?.back) router.back()
  else router.push('/')
}

// Telegram's BackButton replaces in-page back links on nested screens.
let hideBack = () => {}
watch(
  () => route.path,
  (path) => {
    hideBack()
    hideBack = path === '/' ? () => {} : showBackButton(goBack)
  },
  { immediate: true },
)
</script>

<template>
  <AppNav />
  <main class="page">
    <RouterView />
  </main>
</template>
