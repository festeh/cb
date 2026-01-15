<script setup>
const props = defineProps({
  flow: Object
})

const emit = defineEmits(['close', 'save'])

function addStep() {
  props.flow.steps.push({ prompt: '' })
}

function removeStep(idx) {
  if (props.flow.steps.length > 1) {
    props.flow.steps.splice(idx, 1)
  }
}

function addModel() {
  props.flow.models.push('')
}

function removeModel(idx) {
  if (props.flow.models.length > 1) {
    props.flow.models.splice(idx, 1)
  }
}
</script>

<template>
  <div class="modal-overlay" @click.self="emit('close')">
    <div class="modal flow-editor-modal">
      <h2>{{ flow.id ? 'Edit Flow' : 'New Flow' }}</h2>

      <div class="form-group">
        <label>Flow Name</label>
        <input type="text" v-model="flow.name" placeholder="My Flow" />
      </div>

      <div class="form-group">
        <label>Steps</label>
        <div class="steps-list">
          <div v-for="(step, idx) in flow.steps" :key="idx" class="step-item">
            <span class="step-number">{{ idx + 1 }}</span>
            <textarea
              v-model="step.prompt"
              placeholder="Enter prompt for this step..."
              rows="2"
            ></textarea>
            <button
              class="btn-small btn-danger"
              @click="removeStep(idx)"
              :disabled="flow.steps.length <= 1"
            >-</button>
          </div>
          <button class="btn-secondary" @click="addStep">+ Add Step</button>
        </div>
      </div>

      <div class="form-group">
        <label>Models</label>
        <div class="models-list">
          <div v-for="(model, idx) in flow.models" :key="idx" class="model-item">
            <input
              type="text"
              v-model="flow.models[idx]"
              placeholder="anthropic/claude-sonnet-4"
            />
            <button
              class="btn-small btn-danger"
              @click="removeModel(idx)"
              :disabled="flow.models.length <= 1"
            >-</button>
          </div>
          <button class="btn-secondary" @click="addModel">+ Add Model</button>
        </div>
      </div>

      <div class="modal-actions">
        <button class="btn-secondary" @click="emit('close')">Cancel</button>
        <button class="btn-primary" @click="emit('save')">Save Flow</button>
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

.flow-editor-modal {
  max-width: 700px;
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

.form-group input,
.form-group textarea {
  width: 100%;
  padding: 10px 12px;
  background: #0d0d0d;
  border: 1px solid #333;
  border-radius: 6px;
  color: #e0e0e0;
  font-size: 14px;
  font-family: inherit;
}

.form-group input:focus,
.form-group textarea:focus {
  outline: none;
  border-color: #3b82f6;
}

.form-group textarea {
  resize: vertical;
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

.btn-small:disabled {
  opacity: 0.5;
  cursor: default;
}

.btn-danger {
  background: #7f1d1d;
}

.btn-danger:hover {
  background: #991b1b;
}

.steps-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.step-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.step-number {
  width: 24px;
  height: 24px;
  background: #333;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: #888;
  flex-shrink: 0;
  margin-top: 8px;
}

.step-item textarea {
  flex: 1;
}

.models-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.model-item {
  display: flex;
  gap: 8px;
}

.model-item input {
  flex: 1;
}
</style>
