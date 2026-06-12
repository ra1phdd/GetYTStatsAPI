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
    const fieldHints = {
        channelId: "ID вашего YouTube-канала.",
        adWord: "Обязательная строка, которую сервис ищет в описании видео.",
        startDate:
            "Дата старта рекламной компании. Видео раньше этой даты не попадут в результат.",
        endDate:
            "Дата конца рекламной компании. Параметр нужен, чтобы в таблицу не заносились новые видео из другой рекламной компании.",
        hiddenVideos:
            "Необязательный список ID видео через запятую, доступных только по ссылке. Эти видео будут дополнительно запрошены и добавлены в результат.",
    };
    const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL || "http://localhost")
        .trim()
        .replace(/\/$/, "");

    let channelId = "";
    let adWord = "";
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
    let openHint = null;

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
        return (
            columnItems.find((item) => item.key === columnKey)?.enabled ?? false
        );
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
        if (
            channelId.trim() === "" ||
            adWord.trim() === "" ||
            startDate.trim() === ""
        ) {
            copyState = "missing-required";
            setTimeout(() => {
                copyState = "idle";
            }, 2000);
            return;
        }

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

    function toggleHint(hintKey) {
        openHint = openHint === hintKey ? null : hintKey;
    }

    function closeHints() {
        openHint = null;
    }
</script>

<svelte:head>
    <title>GetYTStatsAPI</title>
</svelte:head>

<svelte:window on:click={closeHints} />

<main class="page">
    <header class="page-header" in:fly={{ y: 20, duration: 420 }}>
        <h1>GetYTStatsAPI</h1>
    </header>

    <section class="layout">
        <div class="panel panel-form" in:fade={{ duration: 420 }}>
            <h2>Параметры запроса</h2>

            <div class="field-grid">
                <div class="field">
                    <div class="field-label">
                        <label for="channel-id">Channel ID</label>
                        <button
                            type="button"
                            class="info-tip"
                            class:info-tip--open={openHint === "channelId"}
                            aria-label={fieldHints.channelId}
                            aria-expanded={openHint === "channelId"}
                            on:click|stopPropagation={() =>
                                toggleHint("channelId")}
                        >
                            <span class="info-tip__icon">i</span>
                            <span class="info-tip__bubble"
                                >{fieldHints.channelId}</span
                            >
                        </button>
                    </div>
                    <input
                        id="channel-id"
                        bind:value={channelId}
                        type="text"
                        placeholder="UCNPUUqi4kqjeaScWtsvfyvw"
                    />
                </div>

                <div class="field">
                    <div class="field-label">
                        <label for="ad-word">Ключевое слово рекламы</label>
                        <button
                            type="button"
                            class="info-tip"
                            class:info-tip--open={openHint === "adWord"}
                            aria-label={fieldHints.adWord}
                            aria-expanded={openHint === "adWord"}
                            on:click|stopPropagation={() =>
                                toggleHint("adWord")}
                        >
                            <span class="info-tip__icon">i</span>
                            <span class="info-tip__bubble"
                                >{fieldHints.adWord}</span
                            >
                        </button>
                    </div>
                    <input
                        id="ad-word"
                        bind:value={adWord}
                        type="text"
                        placeholder="w.tv"
                    />
                </div>

                <div class="field">
                    <div class="field-label">
                        <label for="start-date">Дата старта</label>
                        <button
                            type="button"
                            class="info-tip"
                            class:info-tip--open={openHint === "startDate"}
                            aria-label={fieldHints.startDate}
                            aria-expanded={openHint === "startDate"}
                            on:click|stopPropagation={() =>
                                toggleHint("startDate")}
                        >
                            <span class="info-tip__icon">i</span>
                            <span class="info-tip__bubble"
                                >{fieldHints.startDate}</span
                            >
                        </button>
                    </div>
                    <input id="start-date" bind:value={startDate} type="date" />
                </div>

                <div class="field">
                    <div class="field-label">
                        <label for="end-date">Дата окончания</label>
                        <button
                            type="button"
                            class="info-tip"
                            class:info-tip--open={openHint === "endDate"}
                            aria-label={fieldHints.endDate}
                            aria-expanded={openHint === "endDate"}
                            on:click|stopPropagation={() =>
                                toggleHint("endDate")}
                        >
                            <span class="info-tip__icon">i</span>
                            <span class="info-tip__bubble"
                                >{fieldHints.endDate}</span
                            >
                        </button>
                    </div>
                    <input id="end-date" bind:value={endDate} type="date" />
                </div>

                <div class="field field-wide">
                    <div class="field-label">
                        <label for="hidden-videos">Скрытые видео</label>
                        <button
                            type="button"
                            class="info-tip"
                            class:info-tip--open={openHint === "hiddenVideos"}
                            aria-label={fieldHints.hiddenVideos}
                            aria-expanded={openHint === "hiddenVideos"}
                            on:click|stopPropagation={() =>
                                toggleHint("hiddenVideos")}
                        >
                            <span class="info-tip__icon">i</span>
                            <span class="info-tip__bubble"
                                >{fieldHints.hiddenVideos}</span
                            >
                        </button>
                    </div>
                    <input
                        id="hidden-videos"
                        bind:value={hiddenVideos}
                        type="text"
                        placeholder="video1,video2"
                    />
                </div>
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
                use:dragHandleZone={{
                    ...dndZoneOptions,
                    items: displayedColumnItems,
                }}
                on:consider={handleColumnOrderChange}
                on:finalize={handleColumnOrderFinalize}
            >
                {#each displayedColumnItems as column (column.id)}
                    <article
                        class:shadow-item={column[
                            SHADOW_ITEM_MARKER_PROPERTY_NAME
                        ]}
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
                <button
                    type="button"
                    class="ghost-button"
                    class:ghost-button--error={copyState === "missing-required"}
                    on:click={copyFormula}
                >
                    {#if copyState === "copied"}Скопировано{/if}
                    {#if copyState === "error"}Ошибка копирования{/if}
                    {#if copyState === "missing-required"}Заполни обязательные
                        поля{/if}
                    {#if copyState === "idle"}Копировать{/if}
                </button>
            </div>
            <textarea readonly bind:value={formula} rows="6"></textarea>
        </div>
    </section>
</main>
