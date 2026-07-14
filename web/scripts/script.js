document.addEventListener('DOMContentLoaded', () => {
    const searchBox = document.getElementById('search-box');
    const strategySelect = document.getElementById('strategy');
    const searchResults = document.querySelector('.search-results');
    const resultsTableBody = document.querySelector('#results-table tbody');

    // Initially hide the search-results div
    searchResults.style.display = 'none';

    // Function to perform the search
    const performSearch = async () => {
        const query = cleanQuery(searchBox.value.trim());
        const timeTakenDiv = document.getElementById('time-taken');

        if (query.length > 0) {
            try {
                // Start timing
                const startTime = performance.now();

                // Fetch data from the API
                let url = `${searchAPI}?query=${encodeURIComponent(query)}`;
                const strategy = strategySelect.value;
                if (strategy) {
                    url += `&strategy=${encodeURIComponent(strategy)}`;
                }
                const response = await fetch(url);
                if (!response.ok) {
                    const problem = await response.json().catch(() => null);
                    throw new Error(problem && problem.error
                        ? problem.error
                        : 'Failed to fetch search results');
                }

                const data = await response.json();
                // End timing
                const endTime = performance.now();
                const duration = (endTime - startTime).toFixed(2);

                let formattedDuration;
                if (duration >= 60000) {
                    formattedDuration = `${(duration / 60000).toFixed(1)} m`;
                } else if (duration >= 1000) {
                    formattedDuration = `${(duration / 1000).toFixed(1)} s`;
                } else {
                    formattedDuration = `${duration} ms`;
                }

                // Display which configuration answered, and the time taken
                timeTakenDiv.textContent = `${data.resultType} → ${formattedDuration}`;

                // Clear previous results
                resultsTableBody.innerHTML = '';

                // Populate the table with new results
                if (data.matchedProducts && data.matchedProducts.length > 0) {
                    data.matchedProducts.forEach(result => {
                        const row = document.createElement('tr');
                        row.innerHTML = `
                        <td style="border: 1px solid #ddd; padding: 8px;">${escapeHTML(result.class)}</td>
                        <td style="border: 1px solid #ddd; padding: 8px;">${escapeHTML(result.name)}</td>
                        <td style="border: 1px solid #ddd; padding: 8px;">${escapeHTML(truncate(result.description, 220))}</td>
                        <td style="border: 1px solid #ddd; padding: 8px;">${escapeHTML(formatFeatures(result.features))}</td>
                        <td style="border: 1px solid #ddd; padding: 8px; text-align: center;">${formatRating(result.rating)}</td>
                    `;
                        resultsTableBody.appendChild(row);
                    });

                    // Show the search-results div if there are rows
                    searchResults.style.display = 'block';
                } else {
                    // Hide the search-results div if no results
                    searchResults.style.display = 'none';
                    timeTakenDiv.textContent = '';
                }
            } catch (error) {
                console.error('Error fetching search results:', error);
                alert(`Search failed: ${error.message}`);
                timeTakenDiv.textContent = '';
            }
        } else {
            // Hide the search-results div if the query is empty
            searchResults.style.display = 'none';
            timeTakenDiv.textContent = ''; // Clear time if query is empty
        }
    };

    // Debounce function to limit how often performSearch is called
    const debounce = (func, delay) => {
        let timeout;
        return (...args) => {
            clearTimeout(timeout);
            timeout = setTimeout(() => func(...args), delay);
        };
    };

    // Trigger search on input event with debounce
    searchBox.addEventListener('input', debounce(performSearch, 250));

    // Re-run the current search when the strategy changes
    strategySelect.addEventListener('change', performSearch);
});

function cleanQuery(query) {
    return query.replace(/[^a-zA-Z0-9\s]/g, '').trim();
}

function truncate(text, max) {
    if (!text) return '';
    return text.length > max ? `${text.slice(0, max)}…` : text;
}

function formatFeatures(features) {
    if (!features) return '';
    // Features arrive as "name : value | name : value"
    return truncate(features.split('|').slice(0, 3).join(', '), 120);
}

function formatRating(rating) {
    return rating ? Number(rating).toFixed(1) : '—';
}

function escapeHTML(text) {
    const div = document.createElement('div');
    div.textContent = text ?? '';
    return div.innerHTML;
}
