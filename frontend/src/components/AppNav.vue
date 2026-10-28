<script setup>
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
</script>

<template>
  <nav class="nav" aria-label="Main">
    <RouterLink to="/" class="tab" exact-active-class="active">
      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 11h16M6 11v8M18 11v8M8 11V7h8v4" /></svg>
      Venues
    </RouterLink>
    <RouterLink to="/my" class="tab" active-class="active">
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M5 5h14v15H5zM5 9h14M9 3v4M15 3v4" />
      </svg>
      My bookings
    </RouterLink>
    <RouterLink v-if="session.isOwner" to="/owner" class="tab" active-class="active">
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M4 7h16M4 12h16M4 17h10M18 16l1.5 1.5L22 14" />
      </svg>
      Manage
    </RouterLink>
  </nav>
</template>

<style scoped>
.nav {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 10;
  display: flex;
  justify-content: center;
  background: var(--bg);
  border-top: 1px solid var(--separator);
  padding-bottom: env(safe-area-inset-bottom);
}
.tab {
  flex: 1;
  max-width: 214px;
  height: var(--nav-h);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  font-size: 0.72rem;
  font-weight: 500;
  color: var(--hint);
}
.tab svg {
  width: 24px;
  height: 24px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.tab.active {
  color: var(--link);
}
</style>
