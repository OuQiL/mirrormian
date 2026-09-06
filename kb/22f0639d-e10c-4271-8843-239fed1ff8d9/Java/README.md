# Java

## 1. 面向对象核心概念
- **封装、继承、多态**：三大特性。多态的体现（方法重载、重写）、实现机制（动态绑定）。
- **抽象类与接口**：对比（成员变量、方法、构造器、多继承）。Java 8 后接口的默认方法与静态方法。
- **SOLID 原则**：核心设计原则，尤其关注接口隔离、依赖倒置。
- **面试考点**：如何设计一个类体现OOP？抽象类和接口的实际应用场景？举例说明多态。

## 2. JVM 内存模型与垃圾回收
- **运行时数据区**：程序计数器、虚拟机栈、本地方法栈、堆、方法区/元空间的作用与区别。堆内存的分代模型（新生代、老年代）。
- **垃圾回收算法**：标记-清除、复制、标记-整理、分代收集。了解常见的垃圾收集器（如 Serial, Parallel, CMS, G1, ZGC）的特点与适用场景。
- **类加载机制**：双亲委派模型及其破坏场景。
- **面试考点**：描述一个对象从创建到被回收的全过程？常见的内存溢出（OOM）场景及排查思路？如何选择垃圾收集器？

## 3. 多线程与并发编程
- **线程状态与创建**：线程的6种状态转换。创建线程的几种方式（继承Thread、实现Runnable/Callable、线程池）。
- **同步机制**：`synchronized` 关键字（原理、锁升级）、`volatile` 关键字（语义、内存屏障）、`Lock` 接口（可中断、超时、公平锁）。
- **并发工具**：`CountDownLatch`, `CyclicBarrier`, `Semaphore`, `Exchanger` 的作用与区别。
- **线程池**：核心参数（核心线程数、最大线程数、工作队列、拒绝策略）、执行流程、如何合理配置。
- **原子类与CAS**：`java.util.concurrent.atomic` 包下的类原理。
- **面试考点**：`volatile` 与 `synchronized` 的区别？生产者消费者模型的实现？线程池参数如何设置？

## 4. 集合框架
- **核心体系**：`Collection` (List, Set, Queue) 与 `Map` 两大接口族的架构。
- **关键实现**：`ArrayList` vs `LinkedList`（时间复杂度、内存占用）。`HashMap` 底层结构（数组+链表/红黑树）、扩容机制、线程不安全问题。`ConcurrentHashMap` 在 JDK 7 和 8 中的实现差异（分段锁 vs `CAS` + `synchronized`）。
- **面试考点**：`HashMap` 的 `put` 过程？为什么 `HashMap` 的容量是2的幂次？哪些集合是线程安全的？

## 5. 异常处理
- **异常体系**：`Throwable` -> `Error` & `Exception`。`Exception` 分为 `Checked Exception` 和 `Unchecked Exception` (`RuntimeException`)。
- **处理原则**：`try-catch-finally` 的执行顺序。`try-with-resources` 的使用。
- **最佳实践**：异常捕获的粒度、不要吞没异常、自定义异常。
- **面试考点**：`RuntimeException` 和 `Checked Exception` 的区别？`finally` 块中 `return` 的影响？

## 6. I/O 与 NIO
- **BIO（同步阻塞）**：基于流（`InputStream`/`OutputStream`）的模型。
- **NIO（同步非阻塞）**：基于通道（`Channel`）和缓冲区（`Buffer`）。三大核心组件：`Channel`, `Buffer`, `Selector`（多路复用器）。
- **AIO（异步非阻塞）**：基于事件和回调。
- **面试考点**：BIO 与 NIO 的核心区别？NIO 的读写过程？Netty 为什么选择 NIO？

## 7. 泛型
- **核心概念**：类型擦除、泛型通配符 (`?`, `? extends T`, `? super T`)、泛型方法、泛型类。
- **类型擦除**：在编译后被移除，替换为原始类型或 `Object`（有界泛型）。
- **面试考点**：什么是类型擦除？`List` 和 `List<Object>` 的区别？PECS原则（Producer Extends, Consumer Super）的含义和应用。

## 8. 反射与注解
- **反射**：获取 `Class` 对象的三种方式。通过反射创建对象、访问/修改字段、调用方法。性能与安全考虑。
- **注解**：元注解 (`@Target`, `@Retention`, `@Documented`, `@Inherited`)。自定义注解的定义与使用（通常与反射结合）。
- **面试考点**：反射的应用场景（如 Spring IOC）？如何自定义一个注解并实现其功能？

## 9. JVM 调优与性能分析
- **常用工具**：`jps`, `jstack`, `jmap`, `jstat`, `jconsole`, `jvisualvm`, `Arthas`。
- **调优目标**：减少 GC 停顿时间、提高吞吐量、避免内存泄漏。
- **面试考点**：线上应用 CPU 飙高或频繁 Full GC，如何排查？常用 JVM 参数有哪些？

## 10. JDK 新特性（重点版本）
- **JDK 8**：Lambda 表达式、函数式接口、Stream API、Optional、新日期时间 API。
- **JDK 9+**：模块化、`var` 关键字、Record 类（JDK 16）、Sealed Classes（JDK 17）等。
- **面试考点**：Lambda 和匿名类的区别？Stream API 常用操作？`Optional` 的正确用法？