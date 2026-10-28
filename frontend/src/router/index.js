import { createRouter, createWebHistory } from 'vue-router'

import { useSessionStore } from '@/stores/session'

// meta.nav: show the bottom tab bar. meta.back: show an in-page back bar outside Telegram.
const routes = [
  {
    path: '/',
    name: 'venues',
    component: () => import('@/views/customer/VenueList.vue'),
    meta: { nav: true },
  },
  {
    path: '/venues/:id',
    name: 'venue',
    component: () => import('@/views/customer/VenueBooking.vue'),
    props: true,
    meta: { back: true },
  },
  {
    path: '/bookings/:id/done',
    name: 'booking-done',
    component: () => import('@/views/customer/BookingDone.vue'),
    props: true,
    meta: { back: true },
  },
  {
    path: '/my',
    name: 'my-bookings',
    component: () => import('@/views/customer/MyBookings.vue'),
    meta: { nav: true },
  },
  {
    path: '/owner',
    name: 'owner',
    component: () => import('@/views/owner/OwnerHome.vue'),
    meta: { nav: true, requiresOwner: true },
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// UX guard only; the API enforces venue ownership itself.
router.beforeEach(async (to) => {
  if (!to.meta.requiresOwner) return true
  const session = useSessionStore()
  await session.load()
  return session.isOwner ? true : { name: 'venues' }
})

export default router
