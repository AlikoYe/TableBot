<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { getRestaurant, getSlots } from '@/api/restaurants'
import AsyncState from '@/components/AsyncState.vue'
import { useTelegram } from '@/composables/useTelegram'
import { useBookingStore } from '@/stores/booking'
import { formatTime, todayISO } from '@/utils/format'

const props = defineProps({ id: { type: String, required: true } })

const router = useRouter()
const bookingStore = useBookingStore()
const { isTelegram, showMainButton, setMainButtonLoading } = useTelegram()

const restaurant = ref(null)
const date = ref(todayISO())
const guests = ref(2)
const slots = ref([])
const selected = ref(null)
const specialRequest = ref('')

const loading = ref(true)
const error = ref(null)
const submitting = ref(false)
const submitError = ref(null)

async function loadRestaurant() {
  try {
    restaurant.value = await getRestaurant(props.id)
  } catch (err) {
    error.value = err
  }
}

async function loadSlots() {
  loading.value = true
  error.value = null
  selected.value = null
  try {
    slots.value = (await getSlots(props.id, { date: date.value, guests: guests.value })).items
  } catch (err) {
    error.value = err
  } finally {
    loading.value = false
  }
}

async function submit() {
  if (!selected.value || submitting.value) return
  submitting.value = true
  submitError.value = null
  setMainButtonLoading(true)
  try {
    const booking = await bookingStore.create({
      restaurant_id: Number(props.id),
      starts_at: selected.value.starts_at,
      guests: guests.value,
      special_request: specialRequest.value.trim(),
    })
    router.replace({ name: 'booking-done', params: { id: booking.id } })
  } catch (err) {
    submitError.value = err
  } finally {
    submitting.value = false
    setMainButtonLoading(false)
  }
}

// Telegram MainButton mirrors the in-page button; shown once a slot is picked.
const mainButtonText = computed(() =>
  selected.value ? `Book ${formatTime(selected.value.starts_at)} for ${guests.value}` : null,
)
let hideMainButton = () => {}
watch(mainButtonText, (text) => {
  hideMainButton()
  hideMainButton = text ? showMainButton(text, submit) : () => {}
})
onBeforeUnmount(() => hideMainButton())

watch([date, guests], loadSlots)
onMounted(() => Promise.all([loadRestaurant(), loadSlots()]))
</script>

<template>
  <h1>{{ restaurant?.name ?? 'Choose a time' }}</h1>

  <div class="row">
    <label class="field">
      <span>Date</span>
      <input v-model="date" type="date" :min="todayISO()" />
    </label>
    <label class="field">
      <span>Guests</span>
      <select v-model.number="guests">
        <option v-for="n in 10" :key="n" :value="n">{{ n }}</option>
      </select>
    </label>
  </div>

  <AsyncState :loading :error :empty="!slots.length" empty-text="No free tables on this day." @retry="loadSlots">
    <div class="chips">
      <button
        v-for="slot in slots"
        :key="slot.starts_at"
        type="button"
        class="chip"
        :aria-pressed="selected === slot"
        @click="selected = slot"
      >
        {{ formatTime(slot.starts_at) }}
      </button>
    </div>
  </AsyncState>

  <template v-if="selected">
    <label class="field special">
      <span>Special request (optional)</span>
      <textarea v-model="specialRequest" rows="2" maxlength="500" placeholder="High chair, window seat…" />
    </label>
    <p v-if="submitError" class="error">{{ submitError.message }}</p>
    <button v-if="!isTelegram" class="btn btn-block" type="button" :disabled="submitting" @click="submit">
      {{ submitting ? 'Booking…' : mainButtonText }}
    </button>
  </template>
</template>

<style scoped>
.row {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: var(--gap);
}
.special {
  margin-top: 16px;
}
.error {
  color: var(--destructive);
}
</style>
