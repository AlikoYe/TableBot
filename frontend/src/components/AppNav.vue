<script setup>
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
</script>

<template>
  <nav class="nav">
    <RouterLink to="/" class="nav-link" exact-active-class="active">Restaurants</RouterLink>
    <RouterLink to="/my" class="nav-link" active-class="active">My bookings</RouterLink>
    <RouterLink v-if="session.isStaff" to="/staff" class="nav-link" active-class="active">Staff</RouterLink>
  </nav>
</template>

<style scoped>
.nav {
  display: flex;
  gap: 16px;
  padding: 10px 16px;
  background: var(--bg);
  border-bottom: 1px solid var(--separator);
  position: sticky;
  top: 0;
  z-index: 1;
}
.nav-link {
  color: var(--hint);
  font-weight: 500;
}
.nav-link.active {
  color: var(--link);
}
</style>
