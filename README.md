# perspective

perspective is a content management system for websites, written in Go.

## Concepts

* node: a content item, part of the content tree
* slug: name of a node, unique across its siblings
* queue: a stack of slugs
* route: processes a queue
* request: contains a main route (whose queue is the URL) plus zero or more included routes

## License

perspective will be dual-licensed under commercial and open source licenses.

### Contributor License Agreement

By contributing code, you assign its ownership to me. (That is required for commercial licensing.)
