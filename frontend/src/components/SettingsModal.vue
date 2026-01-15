<script setup>
const props = defineProps({
  settings: Object
})

const emit = defineEmits(['close', 'save', 'editFlow', 'deleteFlow', 'createFlow'])
</script>

<template>
  <div class="modal-overlay" @click.self="emit('close')">
    <div class="modal settings-modal">
      <h2>Settings</h2>

      <div class="form-group">
        <label>OpenRouter API Key</label>
        <input
          type="password"
          :value="settings.openrouter_api_key"
          @input="settings.openrouter_api_key = $event.target.value"
          placeholder="sk-or-..."
        />
      </div>

      <div class="form-group">
        <label>Flows</label>
        <div class="flows-list">
          <div v-for="flow in settings.flows" :key="flow.id" class="flow-item">
            <span class="flow-name">{{ flow.name }}</span>
            <span class="flow-info">{{ flow.steps.length }} steps, {{ flow.models.length }} models</span>
            <button class="btn-small" @click="emit('editFlow', flow)">Edit</button>
            <button class="btn-small btn-danger" @click="emit('deleteFlow', flow.id)">Delete</button>
          </div>
          <button class="btn-secondary" @click="emit('createFlow')">+ Add Flow</button>
        </div>
      </div>

      <div class="modal-actions">
        <button class="btn-secondary" @click="emit('close')">Cancel</button>
        <button class="btn-primary" @click="emit('save')">Save</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal {
  background: #1a1a1a;
  border-radius: 12px;
  padding: 24px;
  width: 90%;
  max-width: 500px;
  border: 1px solid #333;
  max-height: 90vh;
  overflow-y: auto;
}

.settings-modal {
  max-width: 600px;
}

.modal h2 {
  margin: 0 0 20px 0;
  font-weight: 500;
  color: #e0e0e0;
}

.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 14px;
  color: #888;
}

.form-group input {
  width: 100%;
  padding: 10px 12px;
  background: #0d0d0d;
  border: 1px solid #333;
  border-radius: 6px;
  color: #e0e0e0;
  font-size: 14px;
}

.form-group input:focus {
  outline: none;
  border-color: #3b82f6;
}

.modal-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  margin-top: 24px;
}

.btn-primary,
.btn-secondary {
  padding: 10px 20px;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
  border: none;
}

.btn-primary {
  background: #3b82f6;
  color: white;
}

.btn-primary:hover {
  background: #2563eb;
}

.btn-secondary {
  background: #333;
  color: #e0e0e0;
}

.btn-secondary:hover {
  background: #444;
}

.btn-small {
  padding: 4px 10px;
  font-size: 12px;
  border-radius: 4px;
  border: none;
  background: #333;
  color: #e0e0e0;
  cursor: pointer;
}

.btn-small:hover {
  background: #444;
}

.btn-danger {
  background: #7f1d1d;
}

.btn-danger:hover {
  background: #991b1b;
}

.flows-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.flow-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  background: #0d0d0d;
  border-radius: 6px;
}

.flow-name {
  font-weight: 500;
  color: #e0e0e0;
}

.flow-info {
  color: #666;
  font-size: 12px;
  flex: 1;
}
</style>
