<script>
    import { flip } from "svelte/animate";
    import {
        dragHandle,
        dragHandleZone,
        SHADOW_ITEM_MARKER_PROPERTY_NAME,
    } from "svelte-dnd-action";
    import { fade, fly, scale } from "svelte/transition";

    const columnOptions = [
        { key: "id", label: "ID" },
        { key: "publish_date", label: "Дата публикации" },
        { key: "video_url", label: "Ссылка на видео" },
        { key: "views", label: "Кол-во просмотров" },
        { key: "ad_timings", label: "Тайминги рекламы" },
        { key: "views_updated_at", label: "Дата обновления просмотров" },
    ];

    const defaultColumns = ["id", "publish_date", "video_url", "views"];
    const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL || "http://localhost")
        .trim()
        .replace(/\/$/, "");

    let channelId = "UCNPUUqi4kqjeaScWtsvfyvw";
    let adWord = "w.tv";
    let startDate = "2026-05-08";
    let endDate = "";
    let hiddenVideos = "";
    let columnItems = createColumnItems();
    let copyState = "idle";
    let displayedColumnItems = [];
    let isDraggingColumns = false;
    let activeColumns = [];
    let statsUrl = "";
    let formula = "";

    const dndZoneOptions = {
        flipDurationMs: 220,
        morphDisabled: true,
        dropTargetStyle: {
            outline: "none",
        },
    };

    $: if (!isDraggingColumns) {
        displayedColumnItems = columnItems;
    }

    $: activeColumns = columnItems
        .filter((item) => item.enabled)
        .map((item) => item.key);

    $: statsUrl = buildStatsUrl(
        channelId,
        adWord,
        startDate,
        endDate,
        hiddenVideos,
        activeColumns,
    );

    $: formula = `=IMPORTDATA("${statsUrl}";",";"en_US")`;

    function createColumnItems() {
        return columnOptions.map((column) => ({
            id: column.key,
            key: column.key,
            label: column.label,
            enabled: defaultColumns.includes(column.key),
        }));
    }

    function isSelected(columnKey) {
        return columnItems.find((item) => item.key === columnKey)?.enabled ?? false;
    }

    function toggleColumn(columnKey) {
        columnItems = columnItems.map((item) =>
            item.key === columnKey ? { ...item, enabled: !item.enabled } : item,
        );
    }

    function resetColumns() {
        columnItems = createColumnItems();
    }

    function handleColumnOrderChange(event) {
        isDraggingColumns = true;
        displayedColumnItems = event.detail.items;
    }

    function handleColumnOrderFinalize(event) {
        displayedColumnItems = event.detail.items;
        columnItems = event.detail.items
            .filter((item) => !item[SHADOW_ITEM_MARKER_PROPERTY_NAME])
            .map((item) => ({
                id: item.id,
                key: item.key,
                label: item.label,
                enabled: item.enabled,
            }));
        isDraggingColumns = false;
    }

    function buildStatsUrl(
        nextChannelId,
        nextAdWord,
        nextStartDate,
        nextEndDate,
        nextHiddenVideos,
        nextActiveColumns,
    ) {
        const params = new URLSearchParams();

        if (nextChannelId.trim() !== "") {
            params.set("channel_id", nextChannelId.trim());
        }
        if (nextAdWord.trim() !== "") {
            params.set("ad_word", nextAdWord.trim());
        }
        if (nextStartDate.trim() !== "") {
            params.set("start_date", nextStartDate.trim());
        }
        if (nextEndDate.trim() !== "") {
            params.set("end_date", nextEndDate.trim());
        }
        if (nextHiddenVideos.trim() !== "") {
            params.set("hidden_videos", nextHiddenVideos.trim());
        }
        if (nextActiveColumns.length > 0) {
            params.set("columns", nextActiveColumns.join(","));
        }

        const query = params.toString();
        if (query === "") {
            return `${apiBaseUrl}/v1/stats/get`;
        }

        return `${apiBaseUrl}/v1/stats/get?${query}`;
    }

    async function copyFormula() {
        try {
            await navigator.clipboard.writeText(formula);
            copyState = "copied";
            setTimeout(() => {
                copyState = "idle";
            }, 1600);
        } catch {
            copyState = "error";
            setTimeout(() => {
                copyState = "idle";
            }, 1600);
        }
    }
</script>

<svelte:head>
    <title>GetYTStatsAPI</title>
    <meta name="description" content="GetYTStatsAPI formula builder." />
</svelte:head>

<main class="page">
    <header class="page-header" in:fly={{ y: 20, duration: 420 }}>
        <h1>GetYTStatsAPI</h1>
        <p class="eyebrow">Google Sheets</p>
    </header>

    <section class="layout">
        <div class="panel panel-form" in:fade={{ duration: 420 }}>
            <h2>Параметры запроса</h2>

            <div class="field-grid">
                <label>
                    <span>Channel ID</span>
                    <input
                        bind:value={channelId}
                        type="text"
                        placeholder="UCNPUUqi4kqjeaScWtsvfyvw"
                    />
                </label>

                <label>
                    <span>Ключевое слово рекламы</span>
                    <input bind:value={adWord} type="text" placeholder="w.tv" />
                </label>

                <label>
                    <span>Дата старта</span>
                    <input bind:value={startDate} type="date" />
                </label>

                <label>
                    <span>Дата окончания</span>
                    <input bind:value={endDate} type="date" />
                </label>

                <label class="field-wide">
                    <span>Скрытые видео</span>
                    <input
                        bind:value={hiddenVideos}
                        type="text"
                        placeholder="video1,video2"
                    />
                </label>
            </div>

        </div>

        <div class="panel panel-columns" in:fade={{ duration: 460, delay: 90 }}>
            <div class="panel-columns__header">
                <div>
                    <h2>Колонки таблицы</h2>
                </div>
                <button
                    type="button"
                    class="ghost-button"
                    on:click={resetColumns}>Reset</button
                >
            </div>

            <div
                class:selected-empty={activeColumns.length === 0}
                class="columns-list columns-list--combined"
                role="list"
                aria-label="Колонки таблицы"
                use:dragHandleZone={{ ...dndZoneOptions, items: displayedColumnItems }}
                on:consider={handleColumnOrderChange}
                on:finalize={handleColumnOrderFinalize}
            >
                {#each displayedColumnItems as column (column.id)}
                    <article
                        class:shadow-item={column[SHADOW_ITEM_MARKER_PROPERTY_NAME]}
                        class:selected={column.enabled}
                        class:column-row--draggable={column.enabled}
                        class="column-row"
                        animate:flip={{ duration: isDraggingColumns ? 220 : 0 }}
                    >
                        <label class="column-check">
                            <input
                                type="checkbox"
                                checked={column.enabled}
                                on:change={() => toggleColumn(column.key)}
                            />
                            <span>{column.label}</span>
                        </label>
                        
                        {#if column.enabled}
                            <div class="column-meta">
                                <div
                                    use:dragHandle
                                    class="drag-handle"
                                    aria-label={`Перетащить колонку ${column.label}`}
                                >
                                    <span></span><span></span><span></span>
                                </div>
                            </div>
                        {/if}
                    </article>
                {/each}
            </div>

            {#if activeColumns.length === 0}
                <div
                    class="empty-state"
                    in:scale={{ duration: 200 }}
                    out:fade={{ duration: 140 }}
                >
                    Выбери хотя бы одну колонку
                </div>
            {/if}
        </div>
    </section>

    <section class="formula-section" in:fade={{ duration: 480, delay: 120 }}>
        <div class="formula-card formula-card--bottom">
            <div class="formula-card__header">
                <span>Готовая формула</span>
                <button type="button" class="ghost-button" on:click={copyFormula}>
                    {#if copyState === "copied"}Скопировано{/if}
                    {#if copyState === "error"}Ошибка копирования{/if}
                    {#if copyState === "idle"}Копировать{/if}
                </button>
            </div>
            <textarea readonly bind:value={formula} rows="6"></textarea>
        </div>
    </section>
</main>
