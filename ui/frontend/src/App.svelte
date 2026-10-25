<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import {
    EventsOn,
    OnFileDrop,
    OnFileDropOff,
  } from "../wailsjs/runtime/runtime";
  import {
    GetFullConfig,
    UpdateAppConfig,
    DeleteAppConfig,
  } from "../wailsjs/go/main/App";
  import type { main } from "../wailsjs/go/models";
  import AgGridSvelte from "./AgGrid.svelte";
  import type { ColDef } from "ag-grid-community";
  import "ag-grid-community/styles/ag-grid.css";
  import "ag-grid-community/styles/ag-theme-alpine.css";

  // ---------------------------------------------------------------------------
  // Types
  // ---------------------------------------------------------------------------
  type AppStateInfo = {
    name: string;
    pid: number;
    state: string;
    enabled: boolean;
    command: string;
    args: string[];
    restartCount: number;
  };

  // ---------------------------------------------------------------------------
  // State: process monitor table
  // ---------------------------------------------------------------------------
  let apps: Record<string, AppStateInfo> = $state({});

  onMount(() => {
    EventsOn("app_state_update", (dataStr: string) => {
      try {
        const data: AppStateInfo = JSON.parse(dataStr);
        if (data.state === "Removed") {
          delete apps[data.name];
          apps = { ...apps };
        } else {
          apps[data.name] = data;
          apps = { ...apps };
        }
      } catch (e) {
        console.error("解析事件失败:", e);
      }
    });

    OnFileDrop((_x: number, _y: number, paths: string[]) => {
      handleDroppedCommand(paths);
    }, false);
  });

  onDestroy(() => {
    OnFileDropOff();
  });

  let appList = $derived(Object.values(apps));

  // ---------------------------------------------------------------------------
  // Cell renderers
  // ---------------------------------------------------------------------------
  function stateRenderer(params: any) {
    if (!params.value) return "";
    let color = "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300";
    const state = params.value.toLowerCase();
    let displayText = params.value;
    
    switch (state) {
      case "running":
        color = "bg-green-100 text-green-800 dark:bg-green-200 dark:text-green-900";
        displayText = "运行中";
        break;
      case "stopped":
        color = "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300";
        displayText = "已停止";
        break;
      case "retrying":
        color = "bg-yellow-100 text-yellow-800 dark:bg-yellow-200 dark:text-yellow-900";
        displayText = "重试中";
        break;
      case "failed":
        color = "bg-red-100 text-red-800 dark:bg-red-200 dark:text-red-900";
        displayText = "已失败";
        break;
    }
    return `<span class="px-2.5 py-0.5 rounded-sm text-xs font-semibold ${color}">${displayText}</span>`;
  }

  function enabledRenderer(params: any) {
    if (params.value === undefined) return "";
    const isEnabled = params.value;
    const color = isEnabled
      ? "bg-blue-100 text-blue-800 dark:bg-blue-200 dark:text-blue-900"
      : "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300";
    return `<span class="px-2.5 py-0.5 rounded-sm text-xs font-semibold ${color}">${isEnabled ? "已启用" : "已禁用"}</span>`;
  }

  function commandRenderer(params: any) {
    const data = params.data;
    if (!data) return "";
    return [data.command, ...(data.args || [])].join(" ");
  }

  let columnDefs: ColDef<AppStateInfo>[] = [
    { field: "name", headerName: "应用名称", flex: 2, minWidth: 150 },
    {
      field: "pid",
      headerName: "PID",
      width: 100,
      valueFormatter: (p: any) => (p.value > 0 ? p.value : "-"),
    },
    {
      field: "state",
      headerName: "状态",
      width: 120,
      cellRenderer: stateRenderer,
    },
    {
      field: "enabled",
      headerName: "启用状态",
      width: 130,
      cellRenderer: enabledRenderer,
    },
    {
      field: "command",
      headerName: "启动命令",
      flex: 3,
      minWidth: 200,
      valueGetter: commandRenderer,
    },
  ];

  // Case-insensitive comparator for string values; falls back to default
  // comparison for non-strings (numbers, booleans, null/undefined).
  function caseInsensitiveComparator(a: unknown, b: unknown): number {
    if (typeof a === "string" && typeof b === "string") {
      return a.localeCompare(b, undefined, { sensitivity: "base" });
    }
    if (a == null && b == null) return 0;
    if (a == null) return -1;
    if (b == null) return 1;
    if (a < b) return -1;
    if (a > b) return 1;
    return 0;
  }

  const defaultColDef: ColDef = {
    resizable: true,
    sortable: true,
    comparator: caseInsensitiveComparator,
  };

  // ---------------------------------------------------------------------------
  // Edit Dialog State
  // ---------------------------------------------------------------------------
  let dialogOpen = $state(false);
  let confirmDelete = $state(false);
  let isSaving = $state(false);
  let isPickingCommand = $state(false);
  let isPickingStopCommand = $state(false);
  let errorMessage = $state("");
  let isCreateMode = $state(false);

  let commandDropExtensions = [".exe", ".cmd", ".bat", ".com", ".ps1", ".vbs"];
  (async () => {
    commandDropExtensions = await getCommandExtensions();
  })();

  let editingOriginalName = $state("");

  let editForm: main.AppConfig = $state(makeEmptyForm());

  function makeEmptyForm(): main.AppConfig {
    return {
      name: "",
      enabled: true,
      command: "",
      args: "",
      stopCommand: "",
      stopArgs: "",
      cwd: "",
      restartDelaySec: 0,
      healthyTimeoutSec: 0,
      hideWindow: false,
      maxRetries: -1,
      stopTimeoutSec: 5,
    } as main.AppConfig;
  }

  async function openEditDialog(appName: string) {
    errorMessage = "";
    confirmDelete = false;
    isSaving = false;
    isCreateMode = false;
    try {
      const configs = await GetFullConfig();
      const cfg = configs[appName];
      if (!cfg) {
        const msg = `在配置中找不到应用 "${appName}"。`;
        errorMessage = msg;
        alert(msg);
        return;
      }
      editingOriginalName = appName;
      editForm = { ...cfg };
      dialogOpen = true;
    } catch (e: any) {
      const msg = `加载配置失败: ${e}`;
      errorMessage = msg;
      alert(msg);
    }
  }

  function openCreateDialog() {
    errorMessage = "";
    confirmDelete = false;
    isSaving = false;
    isCreateMode = true;
    editingOriginalName = "";
    editForm = makeEmptyForm();
    dialogOpen = true;
  }

  function handleRowDoubleClick(event: any) {
    // Ag-grid event data is typically in event.data
    const name: string =
      event?.data?.name ||
      event?.detail?.data?.name ||
      (event?.node && event.node.data && event.node.data.name);

    if (name) {
      openEditDialog(name);
    } else {
      console.error("无法从事件中确定应用名称", event);
    }
  }

  // ---------------------------------------------------------------------------
  // Context Menu State & Handlers
  // ---------------------------------------------------------------------------
  type ContextMenuState = {
    visible: boolean;
    x: number;
    y: number;
    name: string;
    enabled: boolean;
  };

  let contextMenu: ContextMenuState = $state({
    visible: false,
    x: 0,
    y: 0,
    name: "",
    enabled: false,
  });

  let isRestarting: Record<string, boolean> = $state({});

  function handleCellContextMenu(event: any) {
    const data = event?.data;
    if (!data || !data.name) return;

    const mouseEvent = event.event as MouseEvent;
    if (!mouseEvent) return;

    // Window boundaries adjustment
    const menuWidth = 160;
    const menuHeight = 150;
    let x = mouseEvent.clientX;
    let y = mouseEvent.clientY;

    if (x + menuWidth > window.innerWidth) {
      x = Math.max(10, window.innerWidth - menuWidth - 10);
    }
    if (y + menuHeight > window.innerHeight) {
      y = Math.max(10, window.innerHeight - menuHeight - 10);
    }

    contextMenu = {
      visible: true,
      x,
      y,
      name: data.name,
      enabled: data.enabled ?? false,
    };
  }

  function closeContextMenu() {
    contextMenu = { ...contextMenu, visible: false };
  }

  async function handleToggleAppEnabled(appName: string, targetEnabled: boolean) {
    closeContextMenu();
    try {
      const configs = await GetFullConfig();
      const cfg = configs[appName];
      if (!cfg) return;

      await UpdateAppConfig(appName, {
        ...cfg,
        enabled: targetEnabled,
      });
    } catch (e: any) {
      alert(`${targetEnabled ? "启用" : "禁用"} "${appName}" 失败: ${e}`);
    }
  }

  async function handleRestartApp(appName: string) {
    closeContextMenu();
    if (isRestarting[appName]) return;

    isRestarting[appName] = true;
    try {
      const configs = await GetFullConfig();
      const cfg = configs[appName];
      if (!cfg) return;

      // Step 1: Disable
      await UpdateAppConfig(appName, {
        ...cfg,
        enabled: false,
      });

      // Step 2: Wait for core reconciler debounce (500ms in core, wait 800ms)
      await new Promise((resolve) => setTimeout(resolve, 800));

      // Step 3: Enable
      await UpdateAppConfig(appName, {
        ...cfg,
        enabled: true,
      });
    } catch (e: any) {
      alert(`重启 "${appName}" 失败: ${e}`);
    } finally {
      delete isRestarting[appName];
      isRestarting = { ...isRestarting };
    }
  }

  function handleContextEdit(appName: string) {
    closeContextMenu();
    openEditDialog(appName);
  }

  function handleWindowKeydown(e: KeyboardEvent) {
    if (e.key === "Escape" && contextMenu.visible) {
      closeContextMenu();
    }
  }

  function closeDialog() {
    dialogOpen = false;
    confirmDelete = false;
    errorMessage = "";
    isCreateMode = false;
  }

  function getCommandExtensions(): Promise<string[]> {
    return (window as any).go.main.App.GetCommandExtensions();
  }

  function extractAppNameFromPath(filePath: string): string {
    const trimmed = filePath.trim();
    if (!trimmed) return "";

    const segments = trimmed.split(/[/\\]/);
    const filename = segments[segments.length - 1];
    if (!filename) return "";

    const lastDotIndex = filename.lastIndexOf(".");
    if (lastDotIndex > 0) {
      return filename.substring(0, lastDotIndex);
    }
    return filename;
  }

  function setCommandValue(command: string) {
    const updates: Partial<main.AppConfig> = { command };
    if (!editForm.name.trim()) {
      const suggestedName = extractAppNameFromPath(command);
      if (suggestedName) {
        updates.name = suggestedName;
      }
    }
    editForm = { ...editForm, ...updates };
  }

  function isSupportedCommandFile(path: string): boolean {
    const normalized = path.trim().toLowerCase();
    return commandDropExtensions.some((ext) => normalized.endsWith(ext));
  }

  function handleDroppedCommand(paths: string[]) {
    if (!paths || paths.length === 0) {
      return;
    }

    const droppedPath = (paths[0] || "").trim();
    if (!droppedPath) {
      return;
    }

    if (!isSupportedCommandFile(droppedPath)) {
      errorMessage =
        "拖入的文件不是支持的命令类型。请拖入可执行文件或脚本文件。";
      return;
    }

    errorMessage = "";

    if (dialogOpen) {
      confirmDelete = false;
      setCommandValue(droppedPath);
      return;
    }

    openCreateDialog();
    setCommandValue(droppedPath);
  }

  function selectCommandPath(current: string): Promise<string> {
    return (window as any).go.main.App.SelectCommandPath(current);
  }

  async function handleBrowseCommand() {
    errorMessage = "";
    isPickingCommand = true;
    try {
      const selected = await selectCommandPath(editForm.command || "");
      if (selected && selected.trim()) {
        setCommandValue(selected.trim());
      }
    } catch (e: any) {
      errorMessage = `选择命令失败: ${e}`;
    } finally {
      isPickingCommand = false;
    }
  }

  async function handleBrowseStopCommand() {
    errorMessage = "";
    isPickingStopCommand = true;
    try {
      const selected = await selectCommandPath(editForm.stopCommand || "");
      if (selected && selected.trim()) {
        editForm = { ...editForm, stopCommand: selected.trim() };
      }
    } catch (e: any) {
      errorMessage = `选择停止命令失败: ${e}`;
    } finally {
      isPickingStopCommand = false;
    }
  }

  async function handleSave() {
    const targetName = editForm.name.trim();
    const targetCommand = editForm.command.trim();

    if (!targetName) {
      errorMessage = "应用名称不能为空。";
      return;
    }
    if (!targetCommand) {
      errorMessage = "命令不能为空。";
      return;
    }
    isSaving = true;
    errorMessage = "";
    try {
      if (isCreateMode) {
        const configs = await GetFullConfig();
        if (configs[targetName]) {
          errorMessage = `应用 "${targetName}" 已存在。`;
          return;
        }
      }

      await UpdateAppConfig(editingOriginalName, {
        ...editForm,
        name: targetName,
        command: targetCommand,
      });
      closeDialog();
    } catch (e: any) {
      errorMessage = `保存失败: ${e}`;
    } finally {
      isSaving = false;
    }
  }

  function handleDeleteClick() {
    confirmDelete = true;
  }

  async function handleDeleteConfirm() {
    isSaving = true;
    errorMessage = "";
    try {
      await DeleteAppConfig(editingOriginalName);
      closeDialog();
    } catch (e: any) {
      errorMessage = `删除失败: ${e}`;
    } finally {
      isSaving = false;
    }
  }

  function handleDeleteCancel() {
    confirmDelete = false;
  }

  function clampInt(value: number, min: number, max: number): number {
    return Math.min(max, Math.max(min, Math.round(value)));
  }
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<!-- =========================================================================
     Main Layout
     ========================================================================= -->
<main
  class="p-6 h-screen w-full flex flex-col items-center box-border bg-gray-50 dark:bg-slate-900"
>
  <div
    class="w-full flex-1 shadow-2xl sm:rounded-lg flex flex-col overflow-hidden bg-white dark:bg-gray-800"
  >
    <div class="ag-theme-alpine w-full h-full grow">
      <AgGridSvelte
        rowData={appList}
        {columnDefs}
        {defaultColDef}
        rowSelection="single"
        preventDefaultOnContextMenu={true}
        onRowDoubleClicked={handleRowDoubleClick}
        onCellContextMenu={handleCellContextMenu}
        overlayNoRowsTemplate="<span class='text-gray-500 font-medium'>等待进程连接...</span>"
      />
    </div>
    <div class="toolbar">
      <button class="btn btn-primary" type="button" onclick={openCreateDialog}
        >添加</button
      >
    </div>
    <div class="drop-hint">
      将可执行文件拖放到窗口中以注册新的应用条目。
    </div>
  </div>
</main>

<!-- =========================================================================
     Context Menu
     ========================================================================= -->
{#if contextMenu.visible}
  <!-- Backdrop to close context menu on click outside -->
  <div
    class="fixed inset-0 z-40"
    onclick={closeContextMenu}
    oncontextmenu={(e) => {
      e.preventDefault();
      closeContextMenu();
    }}
    role="presentation"
  ></div>

  <!-- Menu Panel -->
  <div
    class="fixed z-50 min-w-[150px] bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-xl py-1 text-sm text-gray-700 dark:text-gray-200 select-none"
    style="top: {contextMenu.y}px; left: {contextMenu.x}px;"
    role="menu"
    tabindex="-1"
  >
    <div
      class="px-3 py-1.5 text-xs font-semibold text-gray-400 dark:text-gray-500 border-b border-gray-100 dark:border-gray-700/60 truncate max-w-[200px]"
    >
      {contextMenu.name}
    </div>
    {#if contextMenu.enabled}
      <button
        class="w-full text-left px-3.5 py-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2 cursor-pointer text-amber-600 dark:text-amber-400"
        onclick={() => handleToggleAppEnabled(contextMenu.name, false)}
        role="menuitem"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        禁用
      </button>
      <button
        class="w-full text-left px-3.5 py-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2 cursor-pointer text-blue-600 dark:text-blue-400"
        onclick={() => handleRestartApp(contextMenu.name)}
        disabled={isRestarting[contextMenu.name]}
        role="menuitem"
      >
        <svg
          class="w-4 h-4 {isRestarting[contextMenu.name] ? 'animate-spin' : ''}"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
          />
        </svg>
        {isRestarting[contextMenu.name] ? "正在重启…" : "重启"}
      </button>
    {:else}
      <button
        class="w-full text-left px-3.5 py-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2 cursor-pointer text-green-600 dark:text-green-400"
        onclick={() => handleToggleAppEnabled(contextMenu.name, true)}
        role="menuitem"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"
          />
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        启用
      </button>
    {/if}
    <div class="h-px bg-gray-100 dark:bg-gray-700/60 my-1"></div>
    <button
      class="w-full text-left px-3.5 py-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2 cursor-pointer text-gray-700 dark:text-gray-200"
      onclick={() => handleContextEdit(contextMenu.name)}
      role="menuitem"
    >
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
        />
      </svg>
      编辑
    </button>
  </div>
{/if}

<!-- =========================================================================
     Edit Dialog
     ========================================================================= -->
{#if dialogOpen}
  <!-- Backdrop -->
  <div class="dialog-backdrop" onclick={closeDialog} role="presentation"></div>

  <!-- Modal -->
  <div
    class="dialog-panel"
    role="dialog"
    aria-modal="true"
    aria-labelledby="dialog-title"
  >
    <div class="dialog-header">
      <h2 id="dialog-title" class="dialog-title">
        {isCreateMode ? "添加应用配置" : "编辑应用配置"}
      </h2>
    </div>

    {#if confirmDelete}
      <!-- ── Delete Confirmation ───────────────────────────────────────── -->
      <div class="confirm-delete-area">
        <div class="confirm-icon">⚠️</div>
        <p class="confirm-text">
          您确定要删除 <strong
            >"{editingOriginalName}"</strong
          > 吗？<br />
          此操作无法撤销。
        </p>
        <div class="confirm-buttons">
          <button
            class="btn btn-ghost"
            onclick={handleDeleteCancel}
            disabled={isSaving}
          >
            取消
          </button>
          <button
            class="btn btn-danger"
            onclick={handleDeleteConfirm}
            disabled={isSaving}
          >
            {isSaving ? "正在删除…" : "是的，删除"}
          </button>
        </div>
      </div>
    {:else}
      <!-- ── Form ──────────────────────────────────────────────────────── -->
      <form
        class="dialog-form"
        onsubmit={(e) => {
          e.preventDefault();
          handleSave();
        }}
      >
        <!-- App Name -->
        <div class="field field-full">
          <label class="field-label" for="field-name"
            >应用名称 <span class="required">*</span></label
          >
          <input
            id="field-name"
            class="field-input"
            type="text"
            bind:value={editForm.name}
            placeholder="我的应用程序"
          />
        </div>

        <!-- Command -->
        <div class="field field-full">
          <label class="field-label" for="field-command"
            >启动命令 <span class="required">*</span></label
          >
          <div class="input-with-button">
            <input
              id="field-command"
              class="field-input field-mono"
              type="text"
              bind:value={editForm.command}
              placeholder="C:\\Windows\\System32\\cmd.exe"
            />
            <button
              type="button"
              class="btn btn-ghost btn-compact"
              onclick={handleBrowseCommand}
              disabled={isSaving || isPickingCommand}
            >
              {isPickingCommand ? "正在打开..." : "浏览..."}
            </button>
          </div>
        </div>

        <!-- Args -->
        <div class="field field-full">
          <label class="field-label" for="field-args">
            参数
            <span class="field-hint"
              >Shell 风格: <code>/c "echo hello" --debug</code></span
            >
          </label>
          <input
            id="field-args"
            class="field-input field-mono"
            type="text"
            bind:value={editForm.args}
            placeholder='/c "我的应用" --verbose'
          />
        </div>

        <!-- CWD -->
        <div class="field field-full">
          <label class="field-label" for="field-cwd">
            工作目录
            <span class="field-hint"
              >可选 - 默认为命令所在目录</span
            >
          </label>
          <input
            id="field-cwd"
            class="field-input field-mono"
            type="text"
            bind:value={editForm.cwd}
            placeholder="C:\path\to\cwd (可选)"
          />
        </div>

        <!-- Checkboxes: Hide Window / Enabled -->
        <div class="checkbox-row">
          <label class="checkbox-label" for="field-hide-window">
            <input
              id="field-hide-window"
              type="checkbox"
              class="checkbox"
              bind:checked={editForm.hideWindow}
            />
            <span>隐藏窗口</span>
          </label>
          <label class="checkbox-label" for="field-enabled">
            <input
              id="field-enabled"
              type="checkbox"
              class="checkbox"
              bind:checked={editForm.enabled}
            />
            <span>启用</span>
          </label>
        </div>

        <!-- Stop Command -->
        <div class="field field-full">
          <label class="field-label" for="field-stop-command">
            停止命令
            <span class="field-hint"
              >可选的可执行文件。留空则自动检测停止。</span
            >
          </label>
          <div class="input-with-button">
            <input
              id="field-stop-command"
              class="field-input field-mono"
              type="text"
              bind:value={editForm.stopCommand}
              placeholder="taskkill"
            />
            <button
              type="button"
              class="btn btn-ghost btn-compact"
              onclick={handleBrowseStopCommand}
              disabled={isSaving || isPickingStopCommand}
            >
              {isPickingStopCommand ? "正在打开..." : "浏览..."}
            </button>
          </div>
        </div>

        <!-- Stop Args -->
        <div class="field field-full">
          <label class="field-label" for="field-stop-args">
            停止参数
            <span class="field-hint"
              >Shell 风格的参数。 <code>{"${PID}"}</code> 将被替换为被监控的 PID。</span
            >
          </label>
          <input
            id="field-stop-args"
            class="field-input field-mono"
            type="text"
            bind:value={editForm.stopArgs}
            placeholder={"/PID ${PID} /T"}
          />
        </div>

        <!-- Numeric fields row: Stop Timeout / Restart Delay -->
        <div class="field-row">
          <div class="field">
            <label class="field-label" for="field-stop-timeout">
              停止超时 (秒)
              <span class="field-hint">默认: 5</span>
            </label>
            <div class="number-input-wrap">
              <input
                id="field-stop-timeout"
                class="field-input field-number"
                type="number"
                min="0"
                max="3600"
                bind:value={editForm.stopTimeoutSec}
                onchange={() =>
                  (editForm.stopTimeoutSec = clampInt(
                    editForm.stopTimeoutSec,
                    0,
                    3600,
                  ))}
              />
            </div>
          </div>
          <div class="field">
            <label class="field-label" for="field-restart-delay">
              重启延迟 (秒)
              <span class="field-hint">默认: 0</span>
            </label>
            <div class="number-input-wrap">
              <input
                id="field-restart-delay"
                class="field-input field-number"
                type="number"
                min="0"
                max="3600"
                bind:value={editForm.restartDelaySec}
                onchange={() =>
                  (editForm.restartDelaySec = clampInt(
                    editForm.restartDelaySec,
                    0,
                    3600,
                  ))}
              />
            </div>
          </div>
        </div>

        <!-- Numeric fields row: Max Retries / Healthy Timeout -->
        <div class="field-row">
          <div class="field">
            <label class="field-label" for="field-max-retries">
              最大重试次数
              <span class="field-hint">0 = 不重试, &lt;0 = 无限重试</span>
            </label>
            <div class="number-input-wrap">
              <input
                id="field-max-retries"
                class="field-input field-number"
                type="number"
                min="-1"
                max="999"
                bind:value={editForm.maxRetries}
                onchange={() =>
                  (editForm.maxRetries = clampInt(
                    editForm.maxRetries,
                    -1,
                    999,
                  ))}
              />
            </div>
          </div>
          <div class="field">
            <label class="field-label" for="field-healthy-timeout">
              健康超时 (秒)
              <span class="field-hint">0 = 不重置运行时间</span>
            </label>
            <div class="number-input-wrap">
              <input
                id="field-healthy-timeout"
                class="field-input field-number"
                type="number"
                min="0"
                max="3600"
                bind:value={editForm.healthyTimeoutSec}
                onchange={() =>
                  (editForm.healthyTimeoutSec = clampInt(
                    editForm.healthyTimeoutSec,
                    0,
                    3600,
                  ))}
              />
            </div>
          </div>
        </div>

        <!-- Error message -->
        {#if errorMessage}
          <div class="error-banner" role="alert">{errorMessage}</div>
        {/if}

        <!-- Action Buttons -->
        <div class="dialog-actions">
          {#if !isCreateMode}
            <button
              type="button"
              class="btn btn-danger-ghost"
              onclick={handleDeleteClick}
              disabled={isSaving}
            >
              删除
            </button>
          {/if}
          <div class="dialog-actions-right">
            <button
              type="button"
              class="btn btn-ghost"
              onclick={closeDialog}
              disabled={isSaving}
            >
              取消
            </button>
            <button type="submit" class="btn btn-primary" disabled={isSaving}>
              {isSaving ? "正在保存…" : "确定"}
            </button>
          </div>
        </div>
      </form>
    {/if}
  </div>
{/if}

<!-- =========================================================================
     Styles (这部分是 CSS，不需要汉化，保持原样即可)
     ========================================================================= -->
<style>
  /* ── Grid ───────────────────────────────────────────────────────────────── */
  :global(.ag-theme-alpine) {
    height: 100% !important;
    width: 100% !important;
  }

  .toolbar {
    display: flex;
    justify-content: flex-end;
    margin-top: 12px;
    padding: 0 12px 12px;
  }

  .drop-hint {
    padding: 0 12px 12px;
    color: #64748b;
    font-size: 0.78rem;
    text-align: right;
  }

  /* ── Backdrop ───────────────────────────────────────────────────────────── */
  .dialog-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    backdrop-filter: blur(4px);
    z-index: 40;
    animation: fadeIn 0.15s ease;
  }

  /* ── Panel ──────────────────────────────────────────────────────────────── */
  .dialog-panel {
    position: fixed;
    top: 50%;
    left: 50%;
    translate: -50% -50%;
    z-index: 50;
    width: min(760px, calc(100vw - 2rem));
    max-height: calc(100vh - 4rem);
    overflow-y: auto;
    border-radius: 16px;
    background: rgba(22, 28, 45, 0.9);
    border: 1px solid rgba(255, 255, 255, 0.12);
    box-shadow:
      0 32px 64px rgba(0, 0, 0, 0.5),
      0 0 0 1px rgba(255, 255, 255, 0.05) inset;
    backdrop-filter: blur(20px) saturate(180%);
    animation: slideUp 0.2s cubic-bezier(0.34, 1.56, 0.64, 1);
    color: #e2e8f0;
    font-family: "Inter", "Segoe UI", sans-serif;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }
  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translate(-50%, calc(-50% + 24px));
    }
    to {
      opacity: 1;
      transform: translate(-50%, -50%);
    }
  }

  /* ── Header ─────────────────────────────────────────────────────────────── */
  .dialog-header {
    padding: 24px 28px 0;
  }
  .dialog-title {
    font-size: 1.2rem;
    font-weight: 700;
    color: #f1f5f9;
    margin: 0 0 4px;
    letter-spacing: -0.02em;
  }
  /* ── Form ───────────────────────────────────────────────────────────────── */
  .dialog-form {
    padding: 20px 28px 24px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 5px;
    flex: 1;
  }
  .field-full {
    width: 100%;
  }
  .field-row {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 12px;
  }

  @media (max-width: 960px) {
    .field-row {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  @media (max-width: 640px) {
    .field-row {
      grid-template-columns: 1fr;
    }
  }

  .field-label {
    font-size: 0.78rem;
    font-weight: 600;
    color: #94a3b8;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    display: flex;
    align-items: baseline;
    gap: 6px;
  }
  .required {
    color: #f87171;
  }
  .field-hint {
    font-size: 0.7rem;
    font-weight: 400;
    text-transform: none;
    letter-spacing: 0;
    color: #4b5563;
  }
  .field-hint code {
    font-family: "JetBrains Mono", monospace;
    background: rgba(255, 255, 255, 0.06);
    padding: 1px 4px;
    border-radius: 3px;
  }

  .field-input {
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 8px 12px;
    color: #f1f5f9;
    font-size: 0.875rem;
    outline: none;
    transition:
      border-color 0.15s,
      box-shadow 0.15s;
    width: 100%;
    box-sizing: border-box;
  }
  .field-input:focus {
    border-color: #6366f1;
    box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.25);
  }
  .field-input::placeholder {
    color: #475569;
  }
  .field-mono {
    font-family: "JetBrains Mono", "Cascadia Code", "Consolas", monospace;
    font-size: 0.82rem;
  }

  .input-with-button {
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .input-with-button .field-input {
    flex: 1;
  }

  /* number input */
  .number-input-wrap {
    position: relative;
  }
  .field-number {
    -moz-appearance: textfield;
    appearance: textfield;
    text-align: center;
    padding-right: 8px;
  }
  .field-number::-webkit-inner-spin-button,
  .field-number::-webkit-outer-spin-button {
    opacity: 1;
    cursor: pointer;
  }

  /* ── Checkboxes ─────────────────────────────────────────────────────────── */
  .checkbox-row {
    display: flex;
    gap: 24px;
    padding: 4px 0;
  }
  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    color: #cbd5e1;
    font-size: 0.88rem;
    font-weight: 500;
    user-select: none;
  }
  .checkbox {
    width: 16px;
    height: 16px;
    cursor: pointer;
    accent-color: #6366f1;
  }

  /* ── Error banner ───────────────────────────────────────────────────────── */
  .error-banner {
    background: rgba(239, 68, 68, 0.15);
    border: 1px solid rgba(239, 68, 68, 0.4);
    border-radius: 8px;
    color: #fca5a5;
    font-size: 0.82rem;
    padding: 8px 12px;
  }

  /* ── Delete confirm ─────────────────────────────────────────────────────── */
  .confirm-delete-area {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    padding: 32px 28px;
    text-align: center;
  }
  .confirm-icon {
    font-size: 2.5rem;
  }
  .confirm-text {
    color: #cbd5e1;
    font-size: 0.95rem;
    line-height: 1.6;
    margin: 0;
  }
  .confirm-text strong {
    color: #f87171;
  }
  .confirm-buttons {
    display: flex;
    gap: 12px;
    margin-top: 8px;
  }

  /* ── Buttons ────────────────────────────────────────────────────────────── */
  .dialog-actions {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    padding-top: 8px;
    gap: 8px;
  }
  .dialog-actions > .btn-danger-ghost {
    margin-right: auto;
  }
  .dialog-actions-right {
    display: flex;
    gap: 8px;
  }

  .btn {
    padding: 8px 18px;
    border-radius: 8px;
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
    border: 1px solid transparent;
    transition:
      background 0.15s,
      border-color 0.15s,
      transform 0.1s,
      opacity 0.15s;
    outline: none;
    white-space: nowrap;
  }
  .btn-compact {
    padding: 8px 12px;
  }
  .btn:active {
    transform: scale(0.97);
  }
  .btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-primary {
    background: #6366f1;
    color: #fff;
    border-color: #6366f1;
  }
  .btn-primary:hover:not(:disabled) {
    background: #4f46e5;
    border-color: #4f46e5;
  }

  .btn-ghost {
    background: transparent;
    color: #94a3b8;
    border-color: rgba(255, 255, 255, 0.1);
  }
  .btn-ghost:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.06);
    color: #e2e8f0;
  }

  .btn-danger {
    background: #ef4444;
    color: #fff;
    border-color: #ef4444;
  }
  .btn-danger:hover:not(:disabled) {
    background: #dc2626;
    border-color: #dc2626;
  }

  .btn-danger-ghost {
    background: transparent;
    color: #f87171;
    border-color: rgba(248, 113, 113, 0.3);
    padding: 8px 14px;
  }
  .btn-danger-ghost:hover:not(:disabled) {
    background: rgba(239, 68, 68, 0.12);
  }
</style>