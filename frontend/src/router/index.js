import { createRouter, createWebHistory } from 'vue-router'

import { useSessionStore } from '@/stores/session'

const routes = [
  {
    path: '/',
    name: 'restaurants',
    component: () => import('@/views/customer/RestaurantList.vue'),
  },
  {
    path: '/restaurants/:id',
    name: 'slot-picker',
    component: () => import('@/views/customer/SlotPicker.vue'),
    props: true,
  },
  {
    path: '/bookings/:id/done',
    name: 'booking-done',
    component: () => import('@/views/customer/BookingDone.vue'),
    props: true,
  },
  {
    path: '/my',
    name: 'my-bookings',
    component: () => import('@/views/customer/MyBookings.vue'),
  },
  {
    path: '/staff',
    name: 'staff-today',
    component: () => import('@/views/staff/TodayBookings.vue'),
    meta: { requiresStaff: true },
  },
  {
    path: '/staff/capacity',
    name: 'staff-capacity',
    component: () => import('@/views/staff/Capacity.vue'),
    meta: { requiresStaff: true },
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// UX guard only; the API enforces staff access itself.
router.beforeEach(async (to) => {
  if (!to.meta.requiresStaff) return true
  const session = useSessionStore()
  await session.load()
  return session.isStaff ? true : { name: 'restaurants' }
})

export default router
