<script setup>
import { onMounted, watch, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AppNav from '@/components/AppNav.vue'
import { useTelegram } from '@/composables/useTelegram'
import { useSessionStore } from '@/stores/session'

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const { isTelegram, colorScheme, showBackButton } = useTelegram()

onMounted(() => session.load())

watchEffect(() => {
  document.documentElement.dataset.scheme = colorScheme.value
})

// Deep links have no in-app history to go back to.
function goBack() {
  if (window.history.state?.back) router.back()
  else router.push('/')
}

// Telegram's BackButton replaces the in-page back bar on nested screens.
let hideBack = () => {}
watch(
  () => route.meta.back,
  (show) => {
    hideBack()
    hideBack = show ? showBackButton(goBack) : () => {}
  },
  { immediate: true },
)
</script>

<template>
  <button v-if="route.meta.back && !isTelegram" class="back" type="button" @click="goBack">
    <span aria-hidden="true">‹</span> Back
  </button>
  <main class="page" :class="{ 'no-nav': !route.meta.nav }">
    <RouterView />
  </main>
  <AppNav v-if="route.meta.nav" />
</template>

<style scoped>
.back {
  display: block;
  width: 100%;
  max-width: 640px;
  margin: 0 auto;
  padding: 12px 16px 0;
  border: 0;
  background: none;
  text-align: left;
  font: inherit;
  font-weight: 600;
  color: var(--link);
  cursor: pointer;
}
.back span {
  font-size: 1.3em;
  line-height: 0;
  margin-right: 2px;
}
</style>
