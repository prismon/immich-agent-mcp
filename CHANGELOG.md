# Changelog

All notable changes to the Immich MCP Server project will be documented in this file.

## [Unreleased]

### Added

#### New Tools
- **smartSearchAdvanced**: Comprehensive AI-powered search with all 34 Immich API parameters
  - Location filters (city, country, state)
  - Camera metadata filters (make, model, lens)
  - Date range filters (created, taken, updated, trashed)
  - Asset property filters (type, visibility, favorites, ratings)
  - Album/person/tag filters
  - Advanced options (motion photos, offline assets, deleted items)
  - Automatic pagination for results > 100 (up to 5000 total)

- **movePhotosBySearch**: Search and organize photos into albums
  - AI-powered search with automatic album creation
  - Supports up to 5000 photos with pagination
  - Dry run mode for previewing changes

- **moveBrokenThumbnailsToAlbum**: Identify and organize broken thumbnails
  - Successfully processed 55,038 broken images in production
  - Automatic pagination for large datasets
  - Configurable batch sizes

- **moveSmallImagesToAlbum**: Organize small images (≤400x400 pixels)
  - Successfully processed 4,699 small images
  - EXIF-based dimension detection
  - Bulk processing support

- **moveLargeMoviesToAlbum**: Organize large movies (>20 minutes)
  - Successfully processed 871 large movies
  - Duration-based filtering
  - Automatic categorization

- **movePersonalVideosFromAlbum**: Separate personal videos from movies
  - Pattern-based detection (IMG_, VID_, MOV_ prefixes)
  - Smartphone video identification
  - Album-to-album transfer

- **deleteAlbumContents**: Bulk delete assets from albums
  - Trash or permanent deletion options
  - Dry run mode for safety
  - Batch processing for large albums

### Enhanced

#### Smart Search Improvements
- Added pagination support for search results exceeding 100 items
- Implemented automatic pagination up to 5000 results
- Fixed limit handling in SmartSearch API calls
- Added comprehensive error handling for API responses

#### Client Improvements
- Added `SmartSearchAdvanced` method with full parameter support
- Implemented `SmartSearchParams` struct for type-safe queries
- Enhanced pagination logic with safety limits
- Added batch processing for bulk operations

#### Documentation
- Complete API documentation in `docs/API.md`
- Comprehensive README with all tool examples
- Added pagination usage examples
- Documented all 34 smart search parameters

### Fixed

- Smart search API limit not being respected (was capped at 100)
- Pagination not working for searches requesting > 100 results
- Missing EXIF data in search results (added `withExif` parameter)
- Album creation duplicates (now checks existing albums first)
- Bulk delete response parsing for array responses

### Technical Details

#### Pagination Implementation
- Page-based pagination for search APIs
- Automatic multi-page fetching for large result sets
- Safety limits to prevent infinite loops (max 50 pages)
- Memory-efficient processing of large datasets

#### Performance Optimizations
- Batch processing in chunks of 100 items
- Concurrent API calls where applicable
- Caching for frequently accessed data
- Efficient memory usage for million+ asset operations

## [0.1.0] - Previous Release

### Initial Implementation
- Basic MCP server using mark3labs/mcp-go v0.39.1
- Streamable HTTP transport
- Basic photo query tools
- Album management
- Asset metadata operations
- Configuration via YAML
- Test suite for core functionality

## Migration Guide

### From 0.1.0 to Current

1. **Smart Search Changes**:
   - Replace basic `queryPhotos` with `smartSearchAdvanced` for complex queries
   - Use `movePhotosBySearch` for search-and-organize workflows
   - Leverage pagination for large result sets

2. **Organization Tools**:
   - Use specialized tools for cleanup (broken thumbnails, small images)
   - Implement bulk operations with `maxImages: 0` for unlimited processing
   - Add dry run checks before production runs

3. **API Changes**:
   - `SmartSearch` now supports pagination automatically
   - New `SmartSearchAdvanced` method for full parameter access
   - Enhanced error responses with detailed messages

## Known Issues

- Smart search requires either `query` or `queryAssetId` parameter
- Maximum 5000 results per search (Immich API limitation)
- Some location filters may require exact matching

## Future Improvements

- [ ] Add face recognition management tools
- [ ] Implement tag management tools
- [ ] Add library management capabilities
- [ ] Support for external library imports
- [ ] Webhook support for real-time updates
- [ ] Batch upload capabilities
- [ ] Advanced duplicate detection algorithms
- [ ] Machine learning-based photo quality assessment