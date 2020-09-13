# perspective

**This is alpha software.**

perspective is a content management system for websites, written in Go.

## Initialize empty database

```
./perspective init -insert -group Admins -user admin
./perspective init -join -group Admins -user admin
./perspective init -make-admin -group Admins
```

## Concepts

* node: a content item, part of the content tree
* slug: name of a node, unique across its siblings
* queue: a stack of slugs
* route: processes a queue
* request: contains a main route (whose queue is the URL) plus zero or more included routes

## License

perspective will be dual-licensed under commercial and open source licenses.

### Contributor License Agreement

By contributing code, you assign its ownership to me. (That is required for dual licensing.)
