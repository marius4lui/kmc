import { readFile, stat } from "node:fs/promises";
import path from "node:path";
import { parseDocument } from "yaml";

const REGISTRY_FIELDS = new Set(["version", "scripts"]);
const REFERENCE_FIELDS = new Set(["file", "description"]);
const WORKFLOW_FIELDS = new Set(["name", "description", "env", "defaults", "steps"]);
const DEFAULT_FIELDS = new Set(["shell", "cwd"]);
const STEP_FIELDS = new Set(["name", "run", "shell", "cwd", "env", "timeout", "retries", "continue_on_error"]);

export class WorkflowValidationError extends Error {
  constructor(errors) {
    super(errors.join("\n"));
    this.name = "WorkflowValidationError";
    this.errors = errors;
  }
}

function object(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function unknownFields(value, allowed, label, errors) {
  if (!object(value)) return;
  for (const field of Object.keys(value)) {
    if (!allowed.has(field)) errors.push(`${label}: field "${field}" is not supported in scripts schema version 1.`);
  }
}

function envErrors(value, label, errors) {
  if (value === undefined) return;
  if (!object(value)) return errors.push(`${label} must be an object.`);
  for (const [key, item] of Object.entries(value)) {
    if (!["string", "number", "boolean"].includes(typeof item) && item !== null) {
      errors.push(`${label}.${key} must be a scalar value.`);
    }
  }
}

function inside(root, target) {
  const relative = path.relative(root, target);
  return relative === "" || (!relative.startsWith("..") && !path.isAbsolute(relative));
}

async function yamlFile(file) {
  let source;
  try {
    source = await readFile(file, "utf8");
  } catch (error) {
    if (error.code === "ENOENT") throw new WorkflowValidationError([`${file}: file was not found.`]);
    throw error;
  }
  if (!source.trim()) throw new WorkflowValidationError([`${file}: file is empty.`]);
  const document = parseDocument(source, { prettyErrors: true, strict: true, uniqueKeys: true });
  if (document.errors.length) {
    throw new WorkflowValidationError(document.errors.map((error) => `${file}: ${error.message}`));
  }
  return document.toJS({ maxAliasCount: 100 });
}

export function validateRegistry(registry, file) {
  const errors = [];
  if (!object(registry)) return [`${file}: registry must be an object.`];
  unknownFields(registry, REGISTRY_FIELDS, file, errors);
  if (registry.version !== 1) errors.push(`${file}: "version" must be 1.`);
  if (!object(registry.scripts)) errors.push(`${file}: "scripts" must be an object.`);
  else for (const [name, reference] of Object.entries(registry.scripts)) {
    const label = `${file}: script "${name}"`;
    if (!name.trim()) errors.push(`${file}: script names must not be empty.`);
    if (!object(reference)) errors.push(`${label} must be an object.`);
    else {
      unknownFields(reference, REFERENCE_FIELDS, label, errors);
      if (typeof reference.file !== "string" || !reference.file.trim()) errors.push(`${label} needs a non-empty "file" string.`);
      if (reference.description !== undefined && typeof reference.description !== "string") errors.push(`${label}: "description" must be a string.`);
    }
  }
  return errors;
}

export function validateWorkflow(workflow, file, projectRoot) {
  const errors = [];
  if (!object(workflow)) return [`${file}: workflow must be an object.`];
  unknownFields(workflow, WORKFLOW_FIELDS, file, errors);
  if (workflow.name !== undefined && typeof workflow.name !== "string") errors.push(`${file}: "name" must be a string.`);
  if (workflow.description !== undefined && typeof workflow.description !== "string") errors.push(`${file}: "description" must be a string.`);
  envErrors(workflow.env, `${file}: env`, errors);
  if (workflow.defaults !== undefined) {
    if (!object(workflow.defaults)) errors.push(`${file}: "defaults" must be an object.`);
    else {
      unknownFields(workflow.defaults, DEFAULT_FIELDS, `${file}: defaults`, errors);
      if (workflow.defaults.shell !== undefined && (typeof workflow.defaults.shell !== "string" || !workflow.defaults.shell.trim())) errors.push(`${file}: defaults.shell must be a non-empty string.`);
      if (workflow.defaults.cwd !== undefined && typeof workflow.defaults.cwd !== "string") errors.push(`${file}: defaults.cwd must be a string.`);
    }
  }
  if (!Array.isArray(workflow.steps) || workflow.steps.length === 0) errors.push(`${file}: "steps" must be a non-empty array.`);
  const names = new Map();
  for (const [index, step] of (Array.isArray(workflow.steps) ? workflow.steps : []).entries()) {
    const label = `${file}: step ${index + 1}`;
    if (!object(step)) { errors.push(`${label} must be an object.`); continue; }
    unknownFields(step, STEP_FIELDS, label, errors);
    if (typeof step.name !== "string" || !step.name.trim()) errors.push(`${label} needs a non-empty "name" string.`);
    else names.set(step.name, (names.get(step.name) ?? 0) + 1);
    if (typeof step.run !== "string" || !step.run.trim()) errors.push(`${label} needs a non-empty "run" string.`);
    if (step.shell !== undefined && (typeof step.shell !== "string" || !step.shell.trim())) errors.push(`${label}: "shell" must be a non-empty string.`);
    if (step.cwd !== undefined && typeof step.cwd !== "string") errors.push(`${label}: "cwd" must be a string.`);
    if (step.timeout !== undefined && (!Number.isInteger(step.timeout) || step.timeout <= 0)) errors.push(`${label}: "timeout" must be a positive integer.`);
    if (step.retries !== undefined && (!Number.isInteger(step.retries) || step.retries < 0)) errors.push(`${label}: "retries" must be a non-negative integer.`);
    if (step.continue_on_error !== undefined && typeof step.continue_on_error !== "boolean") errors.push(`${label}: "continue_on_error" must be a boolean.`);
    envErrors(step.env, `${label}: env`, errors);
  }
  for (const [name, count] of names) if (count > 1) errors.push(`${file}: step name "${name}" is duplicated.`);
  for (const [label, cwd] of [["defaults.cwd", workflow.defaults?.cwd], ...((Array.isArray(workflow.steps) ? workflow.steps : []).map((step, index) => [`step ${index + 1}.cwd`, step?.cwd]))]) {
    if (cwd === undefined) continue;
    if (typeof cwd !== "string") continue;
    const resolved = path.resolve(projectRoot, cwd);
    if (!inside(projectRoot, resolved)) errors.push(`${file}: ${label} escapes the project root.`);
  }
  return errors;
}

export async function loadWorkflows(projectRoot = process.cwd(), { validateCwds = true } = {}) {
  projectRoot = path.resolve(projectRoot);
  const registryFile = path.join(projectRoot, ".kmc", "scripts.yml");
  const registry = await yamlFile(registryFile);
  const errors = validateRegistry(registry, registryFile);
  const scripts = new Map();
  if (errors.length) throw new WorkflowValidationError(errors);
  for (const [id, reference] of Object.entries(registry.scripts)) {
    const file = path.resolve(path.dirname(registryFile), reference.file);
    if (!inside(projectRoot, file)) { errors.push(`${registryFile}: script "${id}" points outside the project root.`); continue; }
    let workflow;
    try { workflow = await yamlFile(file); } catch (error) {
      if (error instanceof WorkflowValidationError) errors.push(...error.errors); else throw error;
      continue;
    }
    errors.push(...validateWorkflow(workflow, file, projectRoot));
    scripts.set(id, { id, description: reference.description ?? workflow.description ?? "", file, workflow });
  }
  if (validateCwds) for (const script of scripts.values()) {
    for (const [index, step] of script.workflow.steps.entries()) {
      const cwd = path.resolve(projectRoot, step.cwd ?? script.workflow.defaults?.cwd ?? ".");
      try {
        if (!(await stat(cwd)).isDirectory()) errors.push(`${script.file}: step ${index + 1} cwd is not a directory: ${cwd}`);
      } catch { errors.push(`${script.file}: step ${index + 1} cwd does not exist: ${cwd}`); }
    }
  }
  if (errors.length) throw new WorkflowValidationError(errors);
  return { projectRoot, registryFile, registry, scripts };
}

export function slug(value) {
  return value.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

export function selectSteps(workflow, selector) {
  if (!selector) return workflow.steps.map((step, index) => ({ ...step, index }));
  const numeric = /^\d+$/.test(selector) ? Number(selector) : null;
  const matches = workflow.steps.map((step, index) => ({ ...step, index })).filter((step) =>
    (numeric !== null && step.index + 1 === numeric) || step.name === selector || slug(step.name) === selector
  );
  if (matches.length !== 1) throw new Error(matches.length ? `Step selector "${selector}" is ambiguous.` : `Step "${selector}" was not found.`);
  return matches;
}
